package repository_test

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"reflect"
	"strings"
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
	"github.com/modernice/goes/aggregate/tagging"
	"github.com/modernice/goes/aggregate/tagging/mock_tagging"
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

func TestRepository_Save_Snapshot(t *testing.T) {
	store := memstore.New()
	snapstore := memsnap.New()
	r := repository.New(
		store,
		repository.WithSnapshots(snapstore, snapshot.Every(3)),
	)

	foo := &mockAggregate{Aggregate: aggregate.New("foo", uuid.New())}
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

func TestRepository_Save_tags(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tagStore := mock_tagging.NewMockStore(ctrl)
	store := memstore.New()
	repo := repository.New(store, repository.WithTags(tagStore))

	foo := newTagger()
	tagging.Tag(foo, "foo", "bar")

	tagStore.EXPECT().
		Tags(gomock.Any(), foo.AggregateName(), foo.AggregateID()).
		DoAndReturn(func(context.Context, string, uuid.UUID) ([]string, error) {
			return []string{}, nil
		})

	tagStore.EXPECT().
		Update(gomock.Any(), foo.AggregateName(), foo.AggregateID(), []string{"bar", "foo"}).
		DoAndReturn(func(context.Context, string, uuid.UUID, []string) error {
			return nil
		})

	if err := repo.Save(context.Background(), foo); err != nil {
		t.Fatalf("Save failed with %q", err)
	}
}

func TestRepository_Save_tagsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tagStore := mock_tagging.NewMockStore(ctrl)
	store := memstore.New()
	repo := repository.New(store, repository.WithTags(tagStore))

	foo := newTagger()
	tagging.Tag(foo, "foo", "bar")

	tagStore.EXPECT().
		Tags(gomock.Any(), foo.AggregateName(), foo.AggregateID()).
		DoAndReturn(func(context.Context, string, uuid.UUID) ([]string, error) {
			return []string{}, nil
		})

	mockError := errors.New("mock error")
	tagStore.EXPECT().
		Update(gomock.Any(), foo.AggregateName(), foo.AggregateID(), []string{"bar", "foo"}).
		DoAndReturn(func(context.Context, string, uuid.UUID, []string) error {
			return mockError
		})

	if err := repo.Save(context.Background(), foo); !errors.Is(err, mockError) {
		t.Fatalf("Save should fail with %q; got %q", mockError, err)
	}

	str, errs, err := store.Query(context.Background(), equery.New())
	if err != nil {
		t.Fatalf("Query failed with %q", err)
	}

	events, err := event.Drain(context.Background(), str, errs)
	if err != nil {
		t.Fatalf("Drain failed with %q", err)
	}

	if len(events) != 0 {
		t.Fatalf("when the tagging store fails to update, no events should be inserted into the event store; got %d", len(events))
	}
}

func TestRepository_Save_tagsRollback(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tagStore := mock_tagging.NewMockStore(ctrl)
	mockError := errors.New("mock error")
	realStore := memstore.New()
	store := newFailingEventStore(realStore, mockError)
	realRepo := repository.New(realStore)
	repo := repository.New(store, repository.WithTags(tagStore))

	foo := newTagger()
	tagging.Tag(foo, "foo", "bar")

	if err := realRepo.Save(context.Background(), foo); err != nil {
		t.Fatalf("failed to save aggregate: %v", err)
	}

	tagging.Tag(foo, "baz")

	tagStore.EXPECT().
		Tags(gomock.Any(), foo.AggregateName(), foo.AggregateID()).
		DoAndReturn(func(context.Context, string, uuid.UUID) ([]string, error) {
			return []string{"foo", "bar"}, nil
		})

	tagStore.EXPECT().
		Update(gomock.Any(), foo.AggregateName(), foo.AggregateID(), []string{"bar", "baz", "foo"}).
		DoAndReturn(func(context.Context, string, uuid.UUID, []string) error {
			return nil
		})

	tagStore.EXPECT().
		Update(gomock.Any(), foo.AggregateName(), foo.AggregateID(), []string{"foo", "bar"}).
		DoAndReturn(func(context.Context, string, uuid.UUID, []string) error {
			return nil
		})

	if err := repo.Save(context.Background(), foo); !errors.Is(err, mockError) {
		t.Fatalf("Save should fail with %q; got %q", mockError, err)
	}
}

func TestRepository_Fetch(t *testing.T) {
	aggregateID := uuid.New()

	org := test.NewFoo(aggregateID)
	aggregate.NextEvent(org, "foo", etest.FooEventData{A: "foo"})
	aggregate.NextEvent(org, "foo", etest.FooEventData{A: "foo"})
	aggregate.NextEvent(org, "foo", etest.FooEventData{A: "foo"})
	events := org.AggregateChanges()

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
	aggregateID := uuid.New()

	org := test.NewFoo(aggregateID)
	aggregate.NextEvent(org, "foo", etest.FooEventData{A: "foo"})
	aggregate.NextEvent(org, "foo", etest.FooEventData{A: "foo"})
	aggregate.NextEvent(org, "foo", etest.FooEventData{A: "foo"})
	aggregate.NextEvent(org, "foo", etest.FooEventData{A: "foo"})
	aggregate.NextEvent(org, "foo", etest.FooEventData{A: "foo"})
	events := org.AggregateChanges()

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
	aggregateID := uuid.New()

	org := test.NewFoo(aggregateID)
	aggregate.NextEvent(org, "foo", etest.FooEventData{A: "foo"})
	aggregate.NextEvent(org, "foo", etest.FooEventData{A: "foo"})
	aggregate.NextEvent(org, "foo", etest.FooEventData{A: "foo"})

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
	aggregate.NextEvent(foo, "foo", etest.FooEventData{A: "foo"})
	aggregate.NextEvent(foo, "foo", etest.FooEventData{A: "foo"})
	aggregate.NextEvent(foo, "foo", etest.FooEventData{A: "foo"})
	changes := foo.AggregateChanges()

	s := memstore.New(changes...)
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

func TestRepository_Delete_tags(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := memstore.New()
	tagStore := mock_tagging.NewMockStore(ctrl)
	nontagRepo := repository.New(store)
	tagRepo := repository.New(store, repository.WithTags(tagStore))

	foo := newTagger()
	tagging.Tag(foo, "foo", "bar")

	if err := nontagRepo.Save(context.Background(), foo); err != nil {
		t.Fatalf("Save failed with %q", err)
	}

	tagStore.EXPECT().
		Update(gomock.Any(), foo.AggregateName(), foo.AggregateID(), []string{}).
		DoAndReturn(func(context.Context, string, uuid.UUID, []string) error {
			return nil
		})

	if err := tagRepo.Delete(context.Background(), foo); err != nil {
		t.Fatalf("Delete failed with %q", err)
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

func TestRepository_Query_tag(t *testing.T) {
	foo := test.NewFoo(uuid.New())

	store := memstore.New(
		tagging.Tag(foo, "foo", "bar"),
		tagging.Tag(foo, "baz"),
	)

	tests := []struct {
		tags      []string
		wantFound bool
	}{
		{
			tags:      []string{"foo"},
			wantFound: true,
		},
		{
			tags:      []string{"bar"},
			wantFound: true,
		},
		{
			tags:      []string{"baz"},
			wantFound: true,
		},
		{
			tags:      []string{"foobar"},
			wantFound: false,
		},
		{
			tags:      nil,
			wantFound: true,
		},
		{
			tags:      []string{},
			wantFound: true,
		},
	}

	for _, tt := range tests {
		t.Run(strings.Join(tt.tags, ", "), func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			tagStore := mock_tagging.NewMockStore(ctrl)

			repo := repository.New(store, repository.WithTags(tagStore))

			if len(tt.tags) > 0 {
				tagStore.EXPECT().
					TaggedWith(gomock.Any(), tt.tags).
					DoAndReturn(func(context.Context, ...string) ([]tagging.Aggregate, error) {
						if tt.wantFound {
							return []tagging.Aggregate{
								{Name: foo.AggregateName(), ID: foo.AggregateID()},
							}, nil
						}
						return nil, nil
					})
			}

			str, errs, err := repo.Query(context.Background(), query.New(query.Tag(tt.tags...)))
			if err != nil {
				t.Fatalf("Query failed with %q", err)
			}

			histories, err := aggregate.Drain(context.Background(), str, errs)
			if err != nil {
				t.Fatalf("Drain failed with %q", err)
			}

			aggregates := make([]aggregate.Aggregate, len(histories))
			for i, his := range histories {
				a := aggregate.New(his.AggregateName(), his.AggregateID())
				his.Apply(a)
				aggregates[i] = a
			}

			if !tt.wantFound {
				if len(aggregates) != 0 {
					t.Fatalf("expected query to return 0 aggregates; got %d", len(aggregates))
				}
				return
			}

			if len(aggregates) != 1 {
				t.Fatalf("expected query to return the aggregate")
			}

			if aggregates[0].AggregateID() != foo.AggregateID() {
				t.Fatalf("query retured wrong aggregate. want=%v got=%v", foo, aggregates[0])
			}
		})
	}
}

func TestRepository_Fetch_Snapshot(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	eventStore := memstore.New()
	mockEventStore := mock_event.NewMockStore(ctrl)
	snapStore := memsnap.New()
	mockStore := mock_snapshot.NewMockStore(ctrl)
	r := repository.New(
		mockEventStore,
		repository.WithSnapshots(mockStore, nil),
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
		DoAndReturn(func(ctx context.Context, q event.Query) (<-chan event.Event, <-chan error, error) {
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

func TestRepository_FetchVersion_Snapshot(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	eventStore := memstore.New()
	mockEventStore := mock_event.NewMockStore(ctrl)
	snapStore := memsnap.New()
	mockStore := mock_snapshot.NewMockStore(ctrl)
	r := repository.New(
		mockEventStore,
		repository.WithSnapshots(mockStore, nil),
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
		DoAndReturn(func(ctx context.Context, q event.Query) (<-chan event.Event, <-chan error, error) {
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
	aggregate.Aggregate

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

type tagger struct {
	*aggregate.Base
	*tagging.Tagger
}

func newTagger() *tagger {
	return &tagger{
		Base:   aggregate.New("foo", uuid.New()),
		Tagger: &tagging.Tagger{},
	}
}

func (t *tagger) ApplyEvent(evt event.Event) {
	t.Tagger.ApplyEvent(evt)
}

type failingEventStore struct {
	event.Store

	err error
}

func newFailingEventStore(store event.Store, err error) *failingEventStore {
	return &failingEventStore{Store: store, err: err}
}

func (s *failingEventStore) Insert(context.Context, ...event.Event) error {
	return s.err
}
