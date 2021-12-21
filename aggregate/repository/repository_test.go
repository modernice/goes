package repository_test

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/query"
	"github.com/modernice/goes/aggregate/repository"
	"github.com/modernice/goes/aggregate/snapshot"
	"github.com/modernice/goes/aggregate/snapshot/memsnap"
	mock_snapshot "github.com/modernice/goes/aggregate/snapshot/mocks"
	squery "github.com/modernice/goes/aggregate/snapshot/query"
	"github.com/modernice/goes/aggregate/test"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventstore"
	mock_event "github.com/modernice/goes/event/mocks"
	equery "github.com/modernice/goes/event/query"
	"github.com/modernice/goes/event/query/version"
	etest "github.com/modernice/goes/event/test"
	"github.com/modernice/goes/internal/xaggregate"
	"github.com/modernice/goes/internal/xevent"
)

func TestRepository_Save(t *testing.T) {
	r := repository.New(eventstore.New())

	aggregateID := uuid.New()
	events := []event.Event{
		event.New("foo", etest.FooEventData{}, event.Aggregate(aggregateID, "foo", 1)),
		event.New("foo", etest.FooEventData{}, event.Aggregate(aggregateID, "foo", 2)),
		event.New("foo", etest.FooEventData{}, event.Aggregate(aggregateID, "foo", 3)),
	}

	flushed := make(chan struct{})
	foo := test.NewFoo(aggregateID, test.CommitFunc(func(flush func()) {
		flush()
		close(flushed)
	}))

	foo.TrackChange(events...)

	if err := r.Save(context.Background(), foo); err != nil {
		t.Fatalf("expected r.Save to succeed; got %#v", err)
	}

	select {
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("didn't flush after %s", 500*time.Millisecond)
	case _, ok := <-flushed:
		if ok {
			t.Fatalf("flushed should be closed")
		}
	}
}

func TestRepository_Save_Snapshot(t *testing.T) {
	store := eventstore.New()
	snapstore := memsnap.New()
	r := repository.New(
		store,
		repository.WithSnapshots(snapstore, snapshot.Every(3)),
	)

	foo := &mockAggregate{Base: aggregate.New("foo", uuid.New())}
	events := xevent.Make("foo", etest.FooEventData{}, 3, xevent.ForAggregate(foo))

	for _, evt := range events {
		foo.ApplyEvent(evt)

		foo.TrackChange(evt)
	}

	if err := r.Save(context.Background(), foo); err != nil {
		t.Fatalf("Save shouldn't fail; failed with %q", err)
	}

	res, errs, err := snapstore.Query(context.Background(), squery.New(
		squery.Name(foo.AggregateName()),
		squery.ID(foo.AggregateID()),
	))
	if err != nil {
		t.Fatalf("Query shouldn't fail; failed with %q", err)
	}

	snaps, err := snapshot.Drain(context.Background(), res, errs)
	if err != nil {
		t.Fatalf("Drain shouldn't fail; failed with %q", err)
	}

	if len(snaps) != 1 {
		t.Fatalf("Query should return exactly 1 Snapshot; got %d", len(snaps))
	}

	snap := snaps[0]
	if snap.AggregateName() != foo.AggregateName() {
		t.Errorf("Snapshot has wrong AggregateName. want=%q got=%q", foo.AggregateName(), snap.AggregateName())
	}
	if snap.AggregateID() != foo.AggregateID() {
		t.Errorf("Snapshot has wrong AggregateID. want=%s got=%s", foo.AggregateID(), snap.AggregateID())
	}
	if snap.AggregateVersion() != foo.AggregateVersion() {
		t.Errorf("Snapshot has wrong AggregateVersion. want=%d got=%d", foo.AggregateVersion(), snap.AggregateVersion())
	}
}

func TestRepository_Fetch(t *testing.T) {
	aggregateID := uuid.New()

	org := test.NewFoo(aggregateID)
	aggregate.NextEvent(org, "foo", etest.FooEventData{A: "foo"})
	aggregate.NextEvent(org, "foo", etest.FooEventData{A: "foo"})
	aggregate.NextEvent(org, "foo", etest.FooEventData{A: "foo"})
	events := org.AggregateChanges()

	r := repository.New(eventstore.New())
	if err := r.Save(context.Background(), org); err != nil {
		t.Fatalf("expected r.Save to succeed; got %#v", err)
	}

	var appliedEvents []event.Event
	var flushed bool
	foo := test.NewFoo(
		aggregateID,
		test.ApplyEventFunc("foo", func(evt event.Event) {
			appliedEvents = append(appliedEvents, evt)
		}),
		test.CommitFunc(func(flush func()) {
			flush()
			flushed = true
		}),
	)
	if err := r.Fetch(context.Background(), foo); err != nil {
		t.Fatalf("expected r.Fetch to succeed; got %#v", err)
	}

	etest.AssertEqualEvents(t, events, appliedEvents)

	if foo.AggregateVersion() != 3 {
		t.Errorf("expected foo.AggregateVersion to return %d; got %d", 3, foo.AggregateVersion())
	}

	if len(foo.AggregateChanges()) != 0 {
		t.Errorf("expected foo to have no changes; got %#v", foo.AggregateChanges())
	}

	if !flushed {
		t.Errorf("expected foo.Commit to have been called")
	}
}

func TestRepository_FetchVersion(t *testing.T) {
	aggregateID := uuid.New()

	org := test.NewFoo(aggregateID)
	aggregate.NextEvent(org, "foo", etest.FooEventData{A: "foo"})
	aggregate.NextEvent(org, "foo", etest.FooEventData{A: "foo"})
	aggregate.NextEvent(org, "foo", etest.FooEventData{A: "foo"})
	aggregate.NextEvent(org, "foo", etest.FooEventData{A: "foo"})
	aggregate.NextEvent(org, "foo", etest.FooEventData{A: "foo"})
	events := org.AggregateChanges()

	r := repository.New(eventstore.New())
	if err := r.Save(context.Background(), org); err != nil {
		t.Fatalf("expected r.Save to succeed; got %#v", err)
	}

	var appliedEvents []event.Event
	var flushed bool
	foo := test.NewFoo(
		aggregateID,
		test.ApplyEventFunc("foo", func(evt event.Event) {
			appliedEvents = append(appliedEvents, evt)
		}),
		test.CommitFunc(func(flush func()) {
			flush()
			flushed = true
		}),
	)
	if err := r.FetchVersion(context.Background(), foo, 4); err != nil {
		t.Fatalf("expected r.FetchVersion to succeed; got %#v", err)
	}

	etest.AssertEqualEvents(t, events[:4], appliedEvents)

	if foo.AggregateVersion() != 4 {
		t.Errorf("expected foo.AggregateVersion to return %d; got %d", 4, foo.AggregateVersion())
	}

	if len(foo.AggregateChanges()) != 0 {
		t.Errorf("expected foo to have no changes; got %#v", foo.AggregateChanges())
	}

	if !flushed {
		t.Errorf("expected foo.Commit to have been called")
	}
}

func TestRepository_FetchVersion_zeroOrNegative(t *testing.T) {
	aggregateName := "foo"
	aggregateID := uuid.New()
	events := []event.Event{
		event.New("foo", etest.FooEventData{A: "foo"}, event.Aggregate(aggregateID, aggregateName, 1)),
		event.New("foo", etest.FooEventData{A: "foo"}, event.Aggregate(aggregateID, aggregateName, 2)),
		event.New("foo", etest.FooEventData{A: "foo"}, event.Aggregate(aggregateID, aggregateName, 3)),
	}

	org := test.NewFoo(aggregateID)
	org.TrackChange(events...)

	r := repository.New(eventstore.New())
	if err := r.Save(context.Background(), org); err != nil {
		t.Fatalf("expected r.Save to succeed; got %#v", err)
	}

	var appliedEvents []event.Event
	foo := test.NewFoo(aggregateID, test.ApplyEventFunc("foo", func(evt event.Event) {
		appliedEvents = append(appliedEvents, evt)
	}))

	if err := r.FetchVersion(context.Background(), foo, -2); err != nil {
		t.Fatalf("r.FetchVersion should not return an error; got %#v", err)
	}

	if len(appliedEvents) > 0 {
		t.Errorf("no events should have been applied")
	}

	if foo.AggregateVersion() != 0 {
		t.Errorf("foo.AggregateVersion should return %d; got %d", 0, foo.AggregateVersion())
	}

	if err := r.FetchVersion(context.Background(), foo, 0); err != nil {
		t.Fatalf("r.FetchVersion should not return an error; got %#v", err)
	}

	if len(appliedEvents) > 0 {
		t.Errorf("no events should have been applied")
	}

	if foo.AggregateVersion() != 0 {
		t.Errorf("foo.AggregateVersion should return %d; got %d", 0, foo.AggregateVersion())
	}
}

func TestRepository_FetchVersion_versionNotReached(t *testing.T) {
	aggregateID := uuid.New()

	org := test.NewFoo(aggregateID)
	aggregate.NextEvent(org, "foo", etest.FooEventData{A: "foo"})
	aggregate.NextEvent(org, "foo", etest.FooEventData{A: "foo"})
	aggregate.NextEvent(org, "foo", etest.FooEventData{A: "foo"})

	r := repository.New(eventstore.New())
	if err := r.Save(context.Background(), org); err != nil {
		t.Fatalf("expected r.Save to succeed; got %#v", err)
	}

	var appliedEvents []event.Event
	foo := test.NewFoo(aggregateID, test.ApplyEventFunc("foo", func(evt event.Event) {
		appliedEvents = append(appliedEvents, evt)
	}))

	if err := r.FetchVersion(context.Background(), foo, 9999); !errors.Is(err, repository.ErrVersionNotFound) {
		t.Fatalf("r.FetchVersion should fail with %q; got %q", repository.ErrVersionNotFound, err)
	}

	if len(appliedEvents) != 3 {
		t.Errorf("3 events should have been applied; got %d", len(appliedEvents))
	}

	if foo.AggregateVersion() != 3 {
		t.Errorf("foo.AggregateVersion should return %d; got %d", 3, foo.AggregateVersion())
	}
}

func TestRepository_Delete(t *testing.T) {
	foo := test.NewFoo(uuid.New())
	aggregate.NextEvent(foo, "foo", etest.FooEventData{A: "foo"})
	aggregate.NextEvent(foo, "foo", etest.FooEventData{A: "foo"})
	aggregate.NextEvent(foo, "foo", etest.FooEventData{A: "foo"})
	changes := foo.AggregateChanges()

	s := eventstore.New(changes...)
	foo = test.NewFoo(foo.AggregateID())
	r := repository.New(s)

	if err := r.Fetch(context.Background(), foo); err != nil {
		t.Fatalf("r.Fetch should not fail: %#v", err)
	}

	if v := foo.AggregateVersion(); v != 3 {
		t.Fatalf("foo.AggregateVersion should return %d; got %d", 3, v)
	}

	if err := r.Delete(context.Background(), foo); err != nil {
		t.Fatalf("r.Delete should not fail: %#v", err)
	}

	foo = test.NewFoo(foo.AggregateID())
	if err := r.Fetch(context.Background(), foo); err != nil {
		t.Fatalf("r.Fetch should not fail: %#v", err)
	}

	if v := foo.AggregateVersion(); v != 0 {
		t.Fatalf("foo.AggregateVersion should return %d; got %d", 0, v)
	}
}

func TestRepository_Query_name(t *testing.T) {
	foos, _ := xaggregate.Make(3, xaggregate.Name("foo"))
	bars, _ := xaggregate.Make(3, xaggregate.Name("bar"))
	bazs, _ := xaggregate.Make(3, xaggregate.Name("baz"))
	as := append(foos, append(bars, bazs...)...)
	am := xaggregate.Map(as)
	events := xevent.Make("foo", etest.FooEventData{}, 10, xevent.ForAggregate(as...))

	s := eventstore.New(events...)
	r := repository.New(s)

	result, err := runQuery(r, query.New(query.Name("foo")), makeFactory(am))
	if err != nil {
		t.Fatal(err)
	}

	foos = aggregate.Sort(foos, aggregate.SortID, aggregate.SortAsc)
	result = aggregate.Sort(result, aggregate.SortID, aggregate.SortAsc)

	if !reflect.DeepEqual(foos, result) {
		t.Fatalf("repository returned the wrong aggregates.\n\nwant: %v\n\ngot: %v", foos, result)
	}

	for _, a := range result {
		aevents := xevent.FilterAggregate(events, a)
		want := event.ExtractAggregateVersion(aevents[len(aevents)-1])
		if aggregate.ExtractVersion(a) != want {
			t.Errorf("aggregate has wrong version. want=%d got=%d", want, aggregate.ExtractVersion(a))
		}
	}
}

func TestRepository_Query_name_multiple(t *testing.T) {
	foos, _ := xaggregate.Make(3, xaggregate.Name("foo"))
	bars, _ := xaggregate.Make(3, xaggregate.Name("bar"))
	bazs, _ := xaggregate.Make(3, xaggregate.Name("baz"))
	as := append(foos, append(bars, bazs...)...)
	am := xaggregate.Map(as)
	events := xevent.Make("foo", etest.FooEventData{}, 10, xevent.ForAggregate(as...))

	s := eventstore.New(events...)
	r := repository.New(s)

	result, err := runQuery(r, query.New(query.Name("foo", "baz")), makeFactory(am))
	if err != nil {
		t.Fatal(err)
	}

	want := append(foos, bazs...)
	want = aggregate.Sort(want, aggregate.SortID, aggregate.SortAsc)
	result = aggregate.Sort(result, aggregate.SortID, aggregate.SortAsc)

	if !reflect.DeepEqual(want, result) {
		t.Fatalf("repository returned the wrong aggregates.\n\nwant: %v\n\ngot: %v", want, result)
	}

	for _, a := range result {
		aevents := xevent.FilterAggregate(events, a)
		want := event.ExtractAggregateVersion(aevents[len(aevents)-1])
		v := aggregate.ExtractVersion(a)
		if v != want {
			t.Errorf("aggregate has wrong version. want=%d got=%d", want, v)
		}
	}
}

func TestRepository_Query_id(t *testing.T) {
	foos, _ := xaggregate.Make(3, xaggregate.Name("foo"))
	bars, _ := xaggregate.Make(3, xaggregate.Name("bar"))
	bazs, _ := xaggregate.Make(3, xaggregate.Name("baz"))
	as := append(foos, append(bars, bazs...)...)
	am := xaggregate.Map(as)
	events := xevent.Make("foo", etest.FooEventData{}, 10, xevent.ForAggregate(as...))

	s := eventstore.New(events...)
	r := repository.New(s)

	result, err := runQuery(r, query.New(
		query.ID(aggregate.ExtractID(foos[0]), aggregate.ExtractID(bazs[2])),
	), makeFactory(am))
	if err != nil {
		t.Fatal(err)
	}

	want := []aggregate.Aggregate{foos[0], bazs[2]}
	want = aggregate.Sort(want, aggregate.SortID, aggregate.SortAsc)
	result = aggregate.Sort(result, aggregate.SortID, aggregate.SortAsc)

	if !reflect.DeepEqual(want, result) {
		t.Fatalf("repository returned the wrong aggregates.\n\nwant: %v\n\ngot: %v", want, result)
	}

	for _, a := range result {
		aevents := xevent.FilterAggregate(events, a)
		want := event.ExtractAggregateVersion(aevents[len(aevents)-1])
		v := aggregate.ExtractVersion(a)
		if v != want {
			t.Errorf("aggregate has wrong version. want=%d got=%d", want, v)
		}
	}
}

func TestRepository_Query_version(t *testing.T) {
	foos, _ := xaggregate.Make(1, xaggregate.Name("foo"))
	bars, _ := xaggregate.Make(1, xaggregate.Name("bar"))
	bazs, _ := xaggregate.Make(1, xaggregate.Name("baz"))
	as := append(foos, append(bars, bazs...)...)
	am := xaggregate.Map(as)
	events := xevent.Make("foo", etest.FooEventData{}, 10, xevent.ForAggregate(as[0]))
	events = append(events, xevent.Make("foo", etest.FooEventData{}, 20, xevent.ForAggregate(as[1]))...)
	events = append(events, xevent.Make("foo", etest.FooEventData{}, 30, xevent.ForAggregate(as[2]))...)
	events = xevent.Shuffle(events)

	s := eventstore.New(events...)
	r := repository.New(s)

	result, err := runQuery(r, query.New(query.Version(version.Exact(10, 20))), makeFactory(am))
	if err != nil {
		t.Fatal(err)
	}

	want := aggregate.Sort(as, aggregate.SortID, aggregate.SortAsc)
	result = aggregate.Sort(result, aggregate.SortID, aggregate.SortAsc)

	if !reflect.DeepEqual(want, result) {
		t.Fatalf("repository returned the wrong aggregates.\n\nwant: %v\n\ngot: %v", want, result)
	}

	for _, a := range result {
		aevents := xevent.FilterAggregate(events, a)
		aevents = event.Sort(aevents, event.SortAggregateVersion, event.SortAsc)
		want := event.ExtractAggregateVersion(aevents[len(aevents)-1])
		if want > 20 {
			want = 20
		}
		v := aggregate.ExtractVersion(a)
		if v != want {
			t.Errorf("aggregate has wrong version. want=%d got=%d", want, v)
		}
	}
}

func TestRepository_Fetch_Snapshot(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	eventStore := eventstore.New()
	mockEventStore := mock_event.NewMockStore(ctrl)
	snapStore := memsnap.New()
	mockStore := mock_snapshot.NewMockStore(ctrl)
	r := repository.New(
		mockEventStore,
		repository.WithSnapshots(mockStore, nil),
	)

	a := &mockAggregate{
		Base: aggregate.New("foo", uuid.New(), aggregate.Version(10)),
	}
	snap, _ := snapshot.New(a)
	a.Base = aggregate.New(a.AggregateName(), a.AggregateID())

	events := xevent.Make("foo", etest.FooEventData{}, 30, xevent.ForAggregate(a))

	if err := eventStore.Insert(context.Background(), events...); err != nil {
		t.Fatalf("failed to insert Events: %v", err)
	}

	if err := snapStore.Save(context.Background(), snap); err != nil {
		t.Fatalf("failed to save Snapshot: %v", err)
	}

	mockStore.EXPECT().
		Latest(gomock.Any(), a.AggregateName(), a.AggregateID()).
		DoAndReturn(func(ctx context.Context, name string, id uuid.UUID) (snapshot.Snapshot, error) {
			return snapStore.Latest(ctx, name, id)
		})

	mockEventStore.EXPECT().
		Query(gomock.Any(), equery.New(
			equery.AggregateName(a.AggregateName()),
			equery.AggregateID(a.AggregateID()),
			equery.AggregateVersion(version.Min(11)),
			equery.SortBy(event.SortAggregateVersion, event.SortAsc),
		)).
		DoAndReturn(func(ctx context.Context, q event.Query) (<-chan event.Event, <-chan error, error) {
			return eventStore.Query(ctx, q)
		})

	res := &mockAggregate{Base: aggregate.New(a.AggregateName(), a.AggregateID())}
	if err := r.Fetch(context.Background(), res); err != nil {
		t.Fatalf("Fetch shouldn't fail; failed with %q", err)
	}

	if res.AggregateVersion() != 30 {
		t.Errorf("Aggregate should have version %d; is %d", 30, res.AggregateVersion())
	}
}

func TestRepository_FetchVersion_Snapshot(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	eventStore := eventstore.New()
	mockEventStore := mock_event.NewMockStore(ctrl)
	snapStore := memsnap.New()
	mockStore := mock_snapshot.NewMockStore(ctrl)
	r := repository.New(
		mockEventStore,
		repository.WithSnapshots(mockStore, nil),
	)

	a := &mockAggregate{Base: aggregate.New("foo", uuid.New(), aggregate.Version(10))}
	snap, _ := snapshot.New(a)
	a.Base = aggregate.New(a.AggregateName(), a.AggregateID())
	events := xevent.Make("foo", etest.FooEventData{}, 30, xevent.ForAggregate(a))

	if err := eventStore.Insert(context.Background(), events...); err != nil {
		t.Fatalf("failed to insert Events: %v", err)
	}

	if err := snapStore.Save(context.Background(), snap); err != nil {
		t.Fatalf("failed to save Snapshot: %v", err)
	}

	mockStore.EXPECT().
		Limit(gomock.Any(), a.AggregateName(), a.AggregateID(), 25).
		DoAndReturn(func(ctx context.Context, name string, id uuid.UUID, v int) (snapshot.Snapshot, error) {
			return snapStore.Limit(ctx, name, id, v)
		})

	mockEventStore.EXPECT().
		Query(gomock.Any(), equery.New(
			equery.AggregateName(a.AggregateName()),
			equery.AggregateID(a.AggregateID()),
			equery.AggregateVersion(version.Min(11), version.Max(25)),
			equery.SortBy(event.SortAggregateVersion, event.SortAsc),
		)).
		DoAndReturn(func(ctx context.Context, q event.Query) (<-chan event.Event, <-chan error, error) {
			return eventStore.Query(ctx, q)
		})

	res := &mockAggregate{Base: aggregate.New(a.AggregateName(), a.AggregateID())}
	if err := r.FetchVersion(context.Background(), res, 25); err != nil {
		t.Fatalf("Fetch shouldn't fail; failed with %q", err)
	}

	if res.AggregateVersion() != 25 {
		t.Errorf("Aggregate should have version %d; is %d", 25, res.AggregateVersion())
	}
}

func TestModifyQueries(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mock_event.NewMockStore(ctrl)

	id := uuid.New()
	repo := repository.New(
		store,
		repository.ModifyQueries(func(_ context.Context, _ aggregate.Query, prev event.Query) (event.Query, error) {
			return equery.Merge(prev, equery.New(
				equery.Aggregate("foo", id),
			)), nil
		}),
	)

	queryChan := make(chan event.Query)

	store.EXPECT().
		Query(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, q event.Query) (<-chan aggregate.History, <-chan error, error) {
			go func() { queryChan <- q }()
			return nil, nil, nil
		})

	ctx, cancel := context.WithCancel(context.Background())
	if _, _, err := repo.Query(ctx, query.New(query.Name("foo"))); err != nil {
		t.Fatalf("Query failed with %q", err)
	}
	cancel()

	q := <-queryChan
	want := equery.New(append(
		query.EventQueryOpts(query.New(query.Name("foo"))),
		equery.Aggregate("foo", id),
		equery.SortByAggregate(),
	)...)
	if !reflect.DeepEqual(q, want) {
		t.Fatalf("event store received the wrong Query.\n\nwant=%v\n\ngot=%v", q, want)
	}
}

func TestBeforeInsert(t *testing.T) {
	store := eventstore.New()
	saved := make(chan aggregate.Aggregate)
	repo := repository.New(store, repository.BeforeInsert(func(_ context.Context, a aggregate.Aggregate) error {
		go func() { saved <- a }()
		return nil
	}))

	foo := test.NewFoo(uuid.New())
	if err := repo.Save(context.Background(), foo); err != nil {
		t.Fatalf("Save failed with %q", err)
	}

	select {
	case <-time.After(time.Second):
		t.Fatal("timed out")
	case saved := <-saved:
		if saved != foo {
			t.Fatalf("BeforeSave received wrong aggregate. want=%v got=%v", foo, saved)
		}
	}
}

func TestAfterInsert(t *testing.T) {
	store := eventstore.New()
	saved := make(chan aggregate.Aggregate)
	repo := repository.New(store, repository.AfterInsert(func(_ context.Context, a aggregate.Aggregate) error {
		go func() { saved <- a }()
		return nil
	}))

	foo := test.NewFoo(uuid.New())
	if err := repo.Save(context.Background(), foo); err != nil {
		t.Fatalf("Save failed with %q", err)
	}

	select {
	case <-time.After(time.Second):
		t.Fatal("timed out")
	case saved := <-saved:
		if saved != foo {
			t.Fatalf("BeforeSave received wrong aggregate. want=%v got=%v", foo, saved)
		}
	}
}

func TestOnFailedInsert(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mock_event.NewMockStore(ctrl)

	gotError := make(chan error)
	gotAggregate := make(chan aggregate.Aggregate)

	mockError := errors.New("mock error")
	store.EXPECT().
		Insert(gomock.Any(), gomock.Any()).
		DoAndReturn(func(context.Context, ...event.Event) error {
			return mockError
		})

	repo := repository.New(store, repository.OnFailedInsert(func(_ context.Context, a aggregate.Aggregate, err error) error {
		go func() {
			gotError <- err
			gotAggregate <- a
		}()
		return nil
	}))

	foo := aggregate.New("foo", uuid.New())

	if err := repo.Save(context.Background(), foo); !errors.Is(err, mockError) {
		t.Fatalf("Save should fail with %q; got %q", mockError, err)
	}

	select {
	case <-time.After(time.Second):
		t.Fatal("timed out")
	case called := <-gotError:
		if !errors.Is(called, mockError) {
			t.Fatalf("OnFailedInsert hook called with wrong error. want=%v got=%v", mockError, called)
		}
	}

	a := <-gotAggregate
	if a != foo {
		t.Fatalf("OnFailedInsert hook called with wrong aggregate. want=%v got=%v", foo, a)
	}
}

func TestOnFailedInsert_error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mock_event.NewMockStore(ctrl)

	mockError := errors.New("mock error")
	store.EXPECT().
		Insert(gomock.Any(), gomock.Any()).
		DoAndReturn(func(context.Context, ...event.Event) error {
			return mockError
		})

	mockHookError := errors.New("mock hook error")
	repo := repository.New(store, repository.OnFailedInsert(func(_ context.Context, a aggregate.Aggregate, err error) error {
		return mockHookError
	}))

	foo := aggregate.New("foo", uuid.New())

	if err := repo.Save(context.Background(), foo); !errors.Is(err, mockHookError) {
		t.Fatalf("Save should fail with %q; got %q", mockHookError, err)
	}
}

func TestOnDelete(t *testing.T) {
	estore := eventstore.New()

	deleted := make(chan aggregate.Aggregate)
	r := repository.New(estore, repository.OnDelete(func(_ context.Context, a aggregate.Aggregate) error {
		go func() { deleted <- a }()
		return nil
	}))

	foo := test.NewFoo(uuid.New())

	if err := r.Delete(context.Background(), foo); err != nil {
		t.Fatalf("Delete failed with %q", err)
	}

	select {
	case <-time.After(time.Second):
		t.Fatal("timed out")
	case del := <-deleted:
		if del != foo {
			t.Fatalf("OnDelete hook called with wrong aggregate. want=%v got=%v", foo, del)
		}
	}
}

func TestOnDelete_error(t *testing.T) {
	estore := eventstore.New()

	mockError := errors.New("mock error")
	r := repository.New(estore, repository.OnDelete(func(_ context.Context, a aggregate.Aggregate) error {
		return mockError
	}))

	foo := test.NewFoo(uuid.New())

	if err := r.Delete(context.Background(), foo); !errors.Is(err, mockError) {
		t.Fatalf("Delete should fail with %q; got %q", mockError, err)
	}
}

func runQuery(r aggregate.Repository, q query.Query, factory func(string, uuid.UUID) aggregate.Aggregate) ([]aggregate.Aggregate, error) {
	appliers, errs, err := r.Query(context.Background(), q)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}

	out := make([]aggregate.Aggregate, 0, len(appliers))
	for {
		select {
		case err, ok := <-errs:
			if !ok {
				errs = nil
				break
			}
			return out, err
		case apply, ok := <-appliers:
			if !ok {
				return out, nil
			}
			a := factory(apply.AggregateName(), apply.AggregateID())
			apply.Apply(a)
			out = append(out, a)
		}
	}
}

func makeFactory(am map[uuid.UUID]aggregate.Aggregate) func(string, uuid.UUID) aggregate.Aggregate {
	return func(_ string, id uuid.UUID) aggregate.Aggregate {
		return am[id]
	}
}

type mockAggregate struct {
	*aggregate.Base

	mockState
}

type mockState struct {
	A string
}

func (a *mockAggregate) MarshalSnapshot() ([]byte, error) {
	var buf bytes.Buffer
	err := gob.NewEncoder(&buf).Encode(a.mockState)
	return buf.Bytes(), err
}

func (a *mockAggregate) UnmarshalSnapshot(p []byte) error {
	return gob.NewDecoder(bytes.NewReader(p)).Decode(&a.mockState)
}
