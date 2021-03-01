package repository_test

import (
	"context"
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
	"github.com/modernice/goes/aggregate/test"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventstore/memstore"
	mock_event "github.com/modernice/goes/event/mocks"
	equery "github.com/modernice/goes/event/query"
	"github.com/modernice/goes/event/query/version"
	etest "github.com/modernice/goes/event/test"
	"github.com/modernice/goes/internal/xaggregate"
	"github.com/modernice/goes/internal/xevent"
)

func TestRepository_Save(t *testing.T) {
	r := repository.New(memstore.New())

	aggregateID := uuid.New()
	events := []event.Event{
		event.New("foo", etest.FooEventData{}, event.Aggregate("foo", aggregateID, 1)),
		event.New("foo", etest.FooEventData{}, event.Aggregate("foo", aggregateID, 2)),
		event.New("foo", etest.FooEventData{}, event.Aggregate("foo", aggregateID, 3)),
	}

	flushed := make(chan struct{})
	foo := test.NewFoo(aggregateID, test.FlushChangesFunc(func(flush func()) {
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

func TestRepository_Fetch(t *testing.T) {
	aggregateName := "foo"
	aggregateID := uuid.New()
	events := []event.Event{
		event.New("foo", etest.FooEventData{A: "foo"}, event.Aggregate(aggregateName, aggregateID, 1)),
		event.New("foo", etest.FooEventData{A: "foo"}, event.Aggregate(aggregateName, aggregateID, 2)),
		event.New("foo", etest.FooEventData{A: "foo"}, event.Aggregate(aggregateName, aggregateID, 3)),
	}

	org := test.NewFoo(aggregateID)
	org.TrackChange(events...)

	r := repository.New(memstore.New())
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
		test.FlushChangesFunc(func(flush func()) {
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
		t.Errorf("expected foo.FlushChanges to have been called")
	}
}

func TestRepository_FetchVersion(t *testing.T) {
	aggregateName := "foo"
	aggregateID := uuid.New()
	events := []event.Event{
		event.New("foo", etest.FooEventData{A: "foo"}, event.Aggregate(aggregateName, aggregateID, 1)),
		event.New("foo", etest.FooEventData{A: "foo"}, event.Aggregate(aggregateName, aggregateID, 2)),
		event.New("foo", etest.FooEventData{A: "foo"}, event.Aggregate(aggregateName, aggregateID, 3)),
		event.New("foo", etest.FooEventData{A: "foo"}, event.Aggregate(aggregateName, aggregateID, 4)),
		event.New("foo", etest.FooEventData{A: "foo"}, event.Aggregate(aggregateName, aggregateID, 5)),
	}

	org := test.NewFoo(aggregateID)
	org.TrackChange(events...)

	r := repository.New(memstore.New())
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
		test.FlushChangesFunc(func(flush func()) {
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
		t.Errorf("expected foo.FlushChanges to have been called")
	}
}

func TestRepository_FetchVersion_zeroOrNegative(t *testing.T) {
	aggregateName := "foo"
	aggregateID := uuid.New()
	events := []event.Event{
		event.New("foo", etest.FooEventData{A: "foo"}, event.Aggregate(aggregateName, aggregateID, 1)),
		event.New("foo", etest.FooEventData{A: "foo"}, event.Aggregate(aggregateName, aggregateID, 2)),
		event.New("foo", etest.FooEventData{A: "foo"}, event.Aggregate(aggregateName, aggregateID, 3)),
	}

	org := test.NewFoo(aggregateID)
	org.TrackChange(events...)

	r := repository.New(memstore.New())
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
	aggregateName := "foo"
	aggregateID := uuid.New()
	events := []event.Event{
		event.New("foo", etest.FooEventData{A: "foo"}, event.Aggregate(aggregateName, aggregateID, 1)),
		event.New("foo", etest.FooEventData{A: "foo"}, event.Aggregate(aggregateName, aggregateID, 2)),
		event.New("foo", etest.FooEventData{A: "foo"}, event.Aggregate(aggregateName, aggregateID, 3)),
	}

	org := test.NewFoo(aggregateID)
	org.TrackChange(events...)

	r := repository.New(memstore.New())
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
	changes := []event.Event{
		event.New("foo", etest.FooEventData{A: "foo"}, event.Aggregate(
			foo.AggregateName(),
			foo.AggregateID(),
			1,
		)),
		event.New("foo", etest.FooEventData{A: "foo"}, event.Aggregate(
			foo.AggregateName(),
			foo.AggregateID(),
			2,
		)),
		event.New("foo", etest.FooEventData{A: "foo"}, event.Aggregate(
			foo.AggregateName(),
			foo.AggregateID(),
			3,
		)),
	}

	s := memstore.New(changes...)
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

	s := memstore.New(events...)
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
		want := aevents[len(aevents)-1].AggregateVersion()
		if a.AggregateVersion() != want {
			t.Errorf("aggregate has wrong version. want=%d got=%d", want, a.AggregateVersion())
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

	s := memstore.New(events...)
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
		want := aevents[len(aevents)-1].AggregateVersion()
		if a.AggregateVersion() != want {
			t.Errorf("aggregate has wrong version. want=%d got=%d", want, a.AggregateVersion())
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

	s := memstore.New(events...)
	r := repository.New(s)

	result, err := runQuery(r, query.New(
		query.ID(foos[0].AggregateID(), bazs[2].AggregateID()),
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
		want := aevents[len(aevents)-1].AggregateVersion()
		if a.AggregateVersion() != want {
			t.Errorf("aggregate has wrong version. want=%d got=%d", want, a.AggregateVersion())
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

	s := memstore.New(events...)
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
		want := aevents[len(aevents)-1].AggregateVersion()
		if want > 20 {
			want = 20
		}
		if a.AggregateVersion() != want {
			t.Errorf("aggregate has wrong version. want=%d got=%d", want, a.AggregateVersion())
		}
	}
}

func TestRepository_Fetch_snapshot(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	eventStore := memstore.New()
	mockEventStore := mock_event.NewMockStore(ctrl)
	snapStore := memsnap.New()
	mockStore := mock_snapshot.NewMockStore(ctrl)
	r := repository.New(
		mockEventStore,
		repository.WithSnapshots(mockStore),
	)

	a := aggregate.New("foo", uuid.New(), aggregate.Version(10))
	snap, _ := snapshot.New(a)
	a = aggregate.New(a.AggregateName(), a.AggregateID())
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
		DoAndReturn(func(ctx context.Context, q event.Query) (event.Stream, error) {
			return eventStore.Query(ctx, q)
		})

	res := aggregate.New(a.AggregateName(), a.AggregateID())
	if err := r.Fetch(context.Background(), res); err != nil {
		t.Fatalf("Fetch shouldn't fail; failed with %q", err)
	}

	if res.AggregateVersion() != 30 {
		t.Errorf("Aggregate should have version %d; is %d", 30, res.AggregateVersion())
	}
}

func TestRepository_FetchVersion_snapshot(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	eventStore := memstore.New()
	mockEventStore := mock_event.NewMockStore(ctrl)
	snapStore := memsnap.New()
	mockStore := mock_snapshot.NewMockStore(ctrl)
	r := repository.New(
		mockEventStore,
		repository.WithSnapshots(mockStore),
	)

	a := aggregate.New("foo", uuid.New(), aggregate.Version(10))
	snap, _ := snapshot.New(a)
	a = aggregate.New(a.AggregateName(), a.AggregateID())
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
		DoAndReturn(func(ctx context.Context, q event.Query) (event.Stream, error) {
			return eventStore.Query(ctx, q)
		})

	res := aggregate.New(a.AggregateName(), a.AggregateID())
	if err := r.FetchVersion(context.Background(), res, 25); err != nil {
		t.Fatalf("Fetch shouldn't fail; failed with %q", err)
	}

	if res.AggregateVersion() != 25 {
		t.Errorf("Aggregate should have version %d; is %d", 25, res.AggregateVersion())
	}
}

func runQuery(r aggregate.Repository, q query.Query, factory func(string, uuid.UUID) aggregate.Aggregate) ([]aggregate.Aggregate, error) {
	str, err := r.Query(context.Background(), q)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer str.Close(context.Background())

	var as []aggregate.Aggregate
	for str.Next(context.Background()) {
		name, id := str.Current()
		a := factory(name, id)
		if err := str.Apply(a); err != nil {
			return as, err
		}
		as = append(as, a)
	}
	return as, str.Err()
}

func makeFactory(am map[uuid.UUID]aggregate.Aggregate) func(string, uuid.UUID) aggregate.Aggregate {
	return func(_ string, id uuid.UUID) aggregate.Aggregate {
		return am[id]
	}
}
