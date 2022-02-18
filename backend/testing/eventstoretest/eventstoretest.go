package eventstoretest

import (
	"context"
	"fmt"
	"testing"
	stdtime "time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/modernice/goes"
	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/query"
	"github.com/modernice/goes/event/query/time"
	"github.com/modernice/goes/event/query/version"
	"github.com/modernice/goes/event/test"
	"github.com/modernice/goes/helper/pick"
	"github.com/modernice/goes/helper/streams"
	"github.com/modernice/goes/internal/xtime"
	"golang.org/x/sync/errgroup"
)

// EventStoreFactory creates an event.Store[ID]
type EventStoreFactory[ID goes.ID] func(codec.Encoding) event.Store[ID]

// Run tests an event store implementation.
func Run[ID goes.ID](t *testing.T, name string, newStore EventStoreFactory[ID], newID func() ID) {
	t.Run(name, func(t *testing.T) {
		run(t, "Insert", newStore, newID, testInsert[ID])
		run(t, "Find", newStore, newID, testFind[ID])
		run(t, "Delete", newStore, newID, testDelete[ID])
		run(t, "Concurrency", newStore, newID, testConcurrency[ID])
		run(t, "Query", newStore, newID, testQuery[ID])
	})
}

func run[ID goes.ID](t *testing.T, name string, newStore EventStoreFactory[ID], newID func() ID, runner func(*testing.T, EventStoreFactory[ID], func() ID)) {
	t.Run(name, func(t *testing.T) {
		runner(t, newStore, newID)
	})
}

func testInsert[ID goes.ID](t *testing.T, newStore EventStoreFactory[ID], newID func() ID) {
	run(t, "SingleInsert", newStore, newID, testSingleInsert[ID])
	run(t, "MultiInsert", newStore, newID, testMultiInsert[ID])
	run(t, "InvalidMultiInsert", newStore, newID, testInvalidMultiInsert[ID])
}

func testSingleInsert[ID goes.ID](t *testing.T, newStore EventStoreFactory[ID], newID func() ID) {
	store := newStore(test.NewEncoder())

	// inserting an event shouldn't fail
	evt := event.New[any](newID(), "foo", test.FooEventData{A: "foo"}, event.Aggregate(newID(), "bar", 3))
	if err := store.Insert(context.Background(), evt); err != nil {
		t.Fatalf("inserting an event shouldn't fail: %v", err)
	}

	// inserting an event with an existing id should fail
	evt = event.New[any](newID(), "foo", test.FooEventData{A: "bar"}, event.ID(evt.ID()))
	if err := store.Insert(context.Background(), evt); err == nil {
		t.Errorf("inserting an event with an existing id should fail; err=%v", err)
	}
}

func testMultiInsert[ID goes.ID](t *testing.T, newStore EventStoreFactory[ID], newID func() ID) {
	store := newStore(test.NewEncoder())
	events := []event.Of[any, ID]{
		event.New[any](newID(), "foo", test.FooEventData{A: "foo"}),
		event.New[any](newID(), "foo", test.FooEventData{A: "foo"}),
		event.New[any](newID(), "foo", test.FooEventData{A: "foo"}),
	}

	if err := store.Insert(context.Background(), events...); err != nil {
		t.Fatalf("expected store.Insert to succeed; got %#v", err)
	}

	result, err := runQuery[ID](store, query.New[ID]())
	if err != nil {
		t.Fatal(err)
	}

	test.AssertEqualEventsUnsorted(t, events, result)
}

// testInvalidMultiInsert tests the uniqueness constraints of the Store returned
// by newStore. We don't test what's actually in the database after the inserts
// because the aggregate repository has the responsibility of validating event
// consistency and doing rollbacks when necessary. Here we just validate basic
// uniqueness constraints that must be ensured on the database level.
func testInvalidMultiInsert[ID goes.ID](t *testing.T, newStore EventStoreFactory[ID], newID func() ID) {
	store := newStore(test.NewEncoder())
	aggregateID := newID()
	eventID := newID()
	events := []event.Of[any, ID]{
		event.New[any](newID(), "foo", test.FooEventData{A: "foo"}, event.Aggregate(aggregateID, "foo", 0)),
		event.New[any](newID(), "foo", test.FooEventData{A: "foo"}, event.Aggregate(aggregateID, "foo", 1)),
		event.New[any](newID(), "foo", test.FooEventData{A: "foo"}, event.Aggregate(aggregateID, "foo", 2)),
		event.New[any](newID(), "foo", test.FooEventData{A: "foo"}, event.Aggregate(aggregateID, "foo", 2)),
		event.New[any](newID(), "foo", test.FooEventData{A: "foo"}, event.Aggregate(aggregateID, "foo", 3), event.ID(eventID)),
		event.New[any](newID(), "foo", test.FooEventData{A: "foo"}, event.Aggregate(aggregateID, "foo", 4), event.ID(eventID)),
	}

	if err := store.Insert(context.Background(), events...); err == nil {
		t.Fatalf("expected store.Insert to fail; got %#v", err)
	}
}

func testFind[ID goes.ID](t *testing.T, newStore EventStoreFactory[ID], newID func() ID) {
	store := newStore(test.NewEncoder())

	found, err := store.Find(context.Background(), newID())
	if err == nil {
		t.Errorf("expected store.Find to return an error; got %#v", err)
	}
	if found != nil {
		t.Errorf("expected store.Find to return no event; got %#v", found)
	}

	evt := event.New[any](newID(), "foo", test.FooEventData{A: "foo"})
	if err := store.Insert(context.Background(), evt); err != nil {
		t.Fatal(fmt.Errorf("store.Insert failed: %w", err))
	}

	found, err = store.Find(context.Background(), evt.ID())
	if err != nil {
		t.Fatalf("expected store.Find not to return error; got %#v", err)
	}

	if !event.Equal(found, evt.Event()) {
		t.Errorf("found event doesn't match inserted event\ninserted: %#v\n\nfound: %#v\n\ndiff: %s", evt, found, cmp.Diff(
			evt, found, cmp.AllowUnexported(evt),
		))
	}
}

func testDelete[ID goes.ID](t *testing.T, newStore EventStoreFactory[ID], newID func() ID) {
	store := newStore(test.NewEncoder())

	foo := event.New[any](newID(), "foo", test.FooEventData{A: "foo"})
	bar := event.New[any](newID(), "bar", test.BarEventData{A: "bar"})

	if err := store.Insert(context.Background(), foo); err != nil {
		t.Fatal(fmt.Errorf("%q: store.Insert failed: %w", "foo", err))
	}

	if err := store.Insert(context.Background(), bar); err != nil {
		t.Fatal(fmt.Errorf("%q: store.Insert failed: %w", "bar", err))
	}

	if err := store.Delete(context.Background(), foo); err != nil {
		t.Fatal(fmt.Errorf("%q: expected store.Delete not to return an error; got %w", "foo", err))
	}

	timeout := stdtime.NewTimer(5 * stdtime.Second)
	defer timeout.Stop()

	ticker := stdtime.NewTicker(500 * stdtime.Millisecond)
	defer ticker.Stop()

L:
	for {
		select {
		case <-timeout.C:
			t.Fatal("timed out. store sill returns deleted event when calling Find()")
		case <-ticker.C:
			found, err := store.Find(context.Background(), foo.ID())
			if err != nil && found == nil {
				break L
			}
		}
	}

	found, err := store.Find(context.Background(), bar.ID())
	if err != nil {
		t.Error(fmt.Errorf("%q: expected store.Find not to return an error; got %#v", "bar", err))
	}
	if !event.Equal(found, bar.Event()) {
		t.Errorf("%q: found event doesn't match inserted event\ninserted: %#v\n\nfound: %#v", "bar", bar, found)
	}
}

func testConcurrency[ID goes.ID](t *testing.T, newStore EventStoreFactory[ID], newID func() ID) {
	run(t, "ConcurrentInsert", newStore, newID, testConcurrentInsert[ID])
	run(t, "ConcurrentFind", newStore, newID, testConcurrentFind[ID])
	run(t, "ConcurrentDelete", newStore, newID, testConcurrentDelete[ID])
}

func testConcurrentInsert[ID goes.ID](t *testing.T, newStore EventStoreFactory[ID], newID func() ID) {
	store := newStore(test.NewEncoder())
	group, ctx := errgroup.WithContext(context.Background())
	for i := 0; i < 30; i++ {
		i := i
		group.Go(func() error {
			evt := event.New[any](newID(), "foo", test.FooEventData{A: "foo"})
			err := store.Insert(ctx, evt)
			if err != nil {
				return fmt.Errorf("[%d] store.Insert failed: %w", i, err)
			}
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		t.Fatal(err)
	}
}

func testConcurrentFind[ID goes.ID](t *testing.T, newStore EventStoreFactory[ID], newID func() ID) {
	store := newStore(test.NewEncoder())
	evt := event.New[any](newID(), "foo", test.FooEventData{A: "foo"})
	if err := store.Insert(context.Background(), evt); err != nil {
		t.Fatal(fmt.Errorf("store.Insert failed: %w", err))
	}

	group, ctx := errgroup.WithContext(context.Background())
	for i := 0; i < 30; i++ {
		i := i
		group.Go(func() error {
			if _, err := store.Find(ctx, evt.ID()); err != nil {
				return fmt.Errorf("[%d] store.Find failed: %w", i, err)
			}
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		t.Fatal(err)
	}
}

func testConcurrentDelete[ID goes.ID](t *testing.T, newStore EventStoreFactory[ID], newID func() ID) {
	store := newStore(test.NewEncoder())

	events := make([]event.Of[any, ID], 30)
	for i := range events {
		events[i] = event.New[any](newID(), "foo", test.FooEventData{A: "foo"})
		if err := store.Insert(context.Background(), events[i]); err != nil {
			t.Fatal(fmt.Errorf("[%d] store.Insert failed: %w", i, err))
		}
	}

	group, ctx := errgroup.WithContext(context.Background())
	for i, evt := range events {
		i := i
		evt := evt
		group.Go(func() error {
			if err := store.Delete(ctx, evt); err != nil {
				return fmt.Errorf("[%d] store.Delete failed: %w", i, err)
			}
			return nil
		})
	}

	if err := group.Wait(); err != nil {
		t.Fatal(err)
	}
}

func testQuery[ID goes.ID](t *testing.T, newStore EventStoreFactory[ID], newID func() ID) {
	run(t, "QueryName", newStore, newID, testQueryName[ID])
	run(t, "QueryID", newStore, newID, testQueryID[ID])
	run(t, "QueryTime", newStore, newID, testQueryTime[ID])
	run(t, "QueryAggregateName", newStore, newID, testQueryAggregateName[ID])
	run(t, "QueryAggregateID", newStore, newID, testQueryAggregateID[ID])
	run(t, "QueryAggregateVersion", newStore, newID, testQueryAggregateVersion[ID])
	run(t, "QueryAggregate", newStore, newID, testQueryAggregate[ID])
	run(t, "Sorting", newStore, newID, testQuerySorting[ID])
}

func testQueryName[ID goes.ID](t *testing.T, newStore EventStoreFactory[ID], newID func() ID) {
	events := []event.Of[any, ID]{
		event.New[any](newID(), "foo", test.FooEventData{A: "foo"}),
		event.New[any](newID(), "foo", test.FooEventData{A: "foo"}, event.Aggregate(newID(), "foo", 10)),
		event.New[any](newID(), "foo", test.FooEventData{A: "foo"}, event.Aggregate(newID(), "foo", 20)),
	}

	// given a store with 3 "foo" events
	store, err := makeStore(newStore, events...)
	if err != nil {
		t.Fatal(err)
	}

	// querying "foo" events should return all 3 events
	result, err := runQuery[ID](store, query.New[ID](query.Name("foo")))
	if err != nil {
		t.Fatal(err)
	}

	test.AssertEqualEventsUnsorted(t, events, result)
}

func testQueryID[ID goes.ID](t *testing.T, newStore EventStoreFactory[ID], newID func() ID) {
	events := []event.Of[any, ID]{
		event.New[any](newID(), "foo", test.FooEventData{A: "foo"}, event.ID(newID())),
		event.New[any](newID(), "bar", test.BarEventData{A: "bar"}, event.ID(newID())),
		event.New[any](newID(), "baz", test.BazEventData{A: "baz"}, event.ID(newID())),
	}

	store, err := makeStore(newStore, events...)
	if err != nil {
		t.Fatal(err)
	}

	q := query.New[ID](query.ID(
		events[0].ID(),
		events[2].ID(),
	))

	result, err := runQuery[ID](store, q)
	if err != nil {
		t.Fatal(err)
	}

	want := []event.Of[any, ID]{events[0], events[2]}
	test.AssertEqualEventsUnsorted(t, want, result)
}

func testQueryTime[ID goes.ID](t *testing.T, newStore EventStoreFactory[ID], newID func() ID) {
	now := xtime.Now()
	events := []event.Of[any, ID]{
		event.New[any](newID(), "foo", test.FooEventData{A: "foo"}, event.Time(now)),
		event.New[any](newID(), "bar", test.BarEventData{A: "bar"}, event.Time(now.AddDate(0, 1, 0))),
		event.New[any](newID(), "baz", test.BazEventData{A: "baz"}, event.Time(now.AddDate(1, 0, 0))),
	}

	store, err := makeStore(newStore, events...)
	if err != nil {
		t.Fatal(err)
	}

	result, err := runQuery[ID](store, query.New[ID](query.Time(
		time.Min(events[1].Time()),
		time.Max(events[2].Time()),
	)))
	if err != nil {
		t.Fatal(err)
	}

	want := events[1:]
	test.AssertEqualEventsUnsorted(t, want, result)
}

func testQueryAggregateName[ID goes.ID](t *testing.T, newStore EventStoreFactory[ID], newID func() ID) {
	events := []event.Of[any, ID]{
		event.New[any](newID(), "foo", test.FooEventData{A: "foo"}),
		event.New[any](newID(), "foo", test.FooEventData{A: "foo"}, event.Aggregate(newID(), "foo", 10)),
		event.New[any](newID(), "foo", test.FooEventData{A: "foo"}, event.Aggregate(newID(), "foo", 20)),
	}

	store, err := makeStore(newStore, events...)
	if err != nil {
		t.Fatal(err)
	}

	result, err := runQuery[ID](store, query.New[ID](query.AggregateName("foo")))
	if err != nil {
		t.Fatal(err)
	}

	want := events[1:]
	test.AssertEqualEventsUnsorted(t, want, result)
}

func testQueryAggregateID[ID goes.ID](t *testing.T, newStore EventStoreFactory[ID], newID func() ID) {
	events := []event.Of[any, ID]{
		event.New[any](newID(), "foo", test.FooEventData{A: "foo"}, event.Aggregate(newID(), "foo", 5)),
		event.New[any](newID(), "foo", test.FooEventData{A: "foo"}, event.Aggregate(newID(), "foo", 10)),
		event.New[any](newID(), "foo", test.FooEventData{A: "foo"}, event.Aggregate(newID(), "foo", 20)),
	}

	store, err := makeStore(newStore, events...)
	if err != nil {
		t.Fatal(err)
	}

	result, err := runQuery[ID](store, query.New[ID](query.AggregateID(
		pick.AggregateID[ID](events[0]),
		pick.AggregateID[ID](events[2]),
	)))
	if err != nil {
		t.Fatal(err)
	}

	want := []event.Of[any, ID]{events[0], events[2]}
	test.AssertEqualEventsUnsorted(t, want, result)
}

func testQueryAggregateVersion[ID goes.ID](t *testing.T, newStore EventStoreFactory[ID], newID func() ID) {
	events := []event.Of[any, ID]{
		event.New[any](newID(), "foo", test.FooEventData{A: "foo"}, event.Aggregate(newID(), "foo", 2)),
		event.New[any](newID(), "foo", test.FooEventData{A: "foo"}, event.Aggregate(newID(), "foo", 4)),
		event.New[any](newID(), "foo", test.FooEventData{A: "foo"}, event.Aggregate(newID(), "foo", 8)),
		event.New[any](newID(), "foo", test.FooEventData{A: "foo"}, event.Aggregate(newID(), "foo", 16)),
		event.New[any](newID(), "foo", test.FooEventData{A: "foo"}, event.Aggregate(newID(), "foo", 32)),
	}

	store, err := makeStore(newStore, events...)
	if err != nil {
		t.Fatal(err)
	}

	result, err := runQuery[ID](store, query.New[ID](query.AggregateVersion(
		version.Min(5),
		version.Max(16),
	)))
	if err != nil {
		t.Fatal(err)
	}

	want := events[2:4]
	test.AssertEqualEventsUnsorted(t, want, result)
}

func testQueryAggregate[ID goes.ID](t *testing.T, newStore EventStoreFactory[ID], newID func() ID) {
	id := newID()
	events := []event.Of[any, ID]{
		event.New[any](newID(), "foo", test.FooEventData{A: "foo"}, event.Aggregate(newID(), "foo", 1)),
		event.New[any](newID(), "foo", test.FooEventData{A: "foo"}, event.Aggregate(id, "foo", 1)),
		event.New[any](newID(), "foo", test.FooEventData{A: "foo"}, event.Aggregate(newID(), "bar", 1)),
		event.New[any](newID(), "foo", test.FooEventData{A: "foo"}, event.Aggregate(id, "bar", 1)),
		event.New[any](newID(), "foo", test.FooEventData{A: "foo"}, event.Aggregate(newID(), "baz", 1)),
		event.New[any](newID(), "foo", test.FooEventData{A: "foo"}, event.Aggregate(id, "baz", 1)),
	}

	store, err := makeStore(newStore, events...)
	if err != nil {
		t.Fatal(err)
	}

	result, err := runQuery[ID](store, query.New[ID](query.Aggregate("foo", id), query.Aggregate("bar", id)))
	if err != nil {
		t.Fatal(err)
	}

	want := []event.Of[any, ID]{events[1], events[3]}
	test.AssertEqualEventsUnsorted(t, result, want)

	result, err = runQuery[ID](store, query.New[ID](query.Aggregate("foo", id), query.Aggregate("baz", uuid.Nil)))
	if err != nil {
		t.Fatal(err)
	}

	want = []event.Of[any, ID]{events[1], events[4], events[5]}
	test.AssertEqualEventsUnsorted(t, result, want)
}

func testQuerySorting[ID goes.ID](t *testing.T, newStore EventStoreFactory[ID], newID func() ID) {
	now := xtime.Now()
	events := []event.Of[any, ID]{
		event.New[any](newID(), "foo", test.FooEventData{A: "foo"}, event.Time(now.Add(12*stdtime.Hour)), event.Aggregate(newID(), "foo1", 3)),
		event.New[any](newID(), "foo", test.FooEventData{A: "foo"}, event.Time(now.Add(stdtime.Hour)), event.Aggregate(newID(), "foo2", 2)),
		event.New[any](newID(), "foo", test.FooEventData{A: "foo"}, event.Time(now.Add(48*stdtime.Hour)), event.Aggregate(newID(), "foo3", 5)),
		event.New[any](newID(), "foo", test.FooEventData{A: "foo"}, event.Time(now), event.Aggregate(newID(), "foo4", 1)),
		event.New[any](newID(), "foo", test.FooEventData{A: "foo"}, event.Time(now.Add(24*stdtime.Hour)), event.Aggregate(newID(), "foo5", 4)),
	}

	tests := []struct {
		name string
		q    query.Query[ID]
		want []event.Of[any, ID]
	}{
		{
			name: "SortTime(asc)",
			q:    query.New[ID](query.SortBy(event.SortTime, event.SortAsc)),
			want: event.Sort(events, event.SortTime, event.SortAsc),
		},
		{
			name: "SortTime(desc)",
			q:    query.New[ID](query.SortBy(event.SortTime, event.SortDesc)),
			want: event.Sort(events, event.SortTime, event.SortDesc),
		},
		{
			name: "Time+SortTime(asc)",
			q: query.New[ID](
				query.Time(
					time.Min(events[0].Time()),
					time.Max(events[4].Time()),
				),
				query.SortBy(event.SortTime, event.SortAsc),
			),
			want: event.Sort([]event.Of[any, ID]{events[0], events[4]}, event.SortTime, event.SortAsc),
		},
		{
			name: "Time+SortTime(desc)",
			q: query.New[ID](
				query.Time(
					time.Min(events[0].Time()),
					time.Max(events[4].Time()),
				),
				query.SortBy(event.SortTime, event.SortDesc),
			),
			want: event.Sort([]event.Of[any, ID]{events[0], events[4]}, event.SortTime, event.SortDesc),
		},
		{
			name: "SortAggregateName(asc)",
			q:    query.New[ID](query.SortBy(event.SortAggregateName, event.SortAsc)),
			want: event.Sort(events, event.SortAggregateName, event.SortAsc),
		},
		{
			name: "SortAggregateName(desc)",
			q:    query.New[ID](query.SortBy(event.SortAggregateName, event.SortDesc)),
			want: event.Sort(events, event.SortAggregateName, event.SortDesc),
		},
		{
			name: "SortAggregateID(asc)",
			q:    query.New[ID](query.SortBy(event.SortAggregateID, event.SortAsc)),
			want: event.Sort(events, event.SortAggregateID, event.SortAsc),
		},
		{
			name: "SortAggregateID(desc)",
			q:    query.New[ID](query.SortBy(event.SortAggregateID, event.SortDesc)),
			want: event.Sort(events, event.SortAggregateID, event.SortDesc),
		},
		{
			name: "SortAggregateVersion(asc)",
			q:    query.New[ID](query.SortBy(event.SortAggregateVersion, event.SortAsc)),
			want: event.Sort(events, event.SortAggregateVersion, event.SortAsc),
		},
		{
			name: "SortAggregateVersion(desc)",
			q:    query.New[ID](query.SortBy(event.SortAggregateVersion, event.SortDesc)),
			want: event.Sort(events, event.SortAggregateVersion, event.SortDesc),
		},
		{
			name: "SortAggregateName(desc)+SortAggregateVersion(asc)",
			q: query.New[ID](
				query.SortByMulti(event.SortOptions{Sort: event.SortAggregateName, Dir: event.SortDesc}),
				query.SortByMulti(event.SortOptions{Sort: event.SortAggregateVersion, Dir: event.SortAsc}),
			),
			want: event.SortMulti(
				events,
				event.SortOptions{Sort: event.SortAggregateName, Dir: event.SortDesc},
				event.SortOptions{Sort: event.SortAggregateVersion, Dir: event.SortAsc},
			),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, err := makeStore(newStore, events...)
			if err != nil {
				t.Fatal(err)
			}

			result, err := runQuery[ID](store, tt.q)
			if err != nil {
				t.Fatalf("expected query to succeed: %#v", err)
			}

			test.AssertEqualEvents(t, tt.want, result)
		})
	}
}

func makeStore[ID goes.ID](newStore EventStoreFactory[ID], events ...event.Of[any, ID]) (event.Store[ID], error) {
	store := newStore(test.NewEncoder())
	for i, evt := range events {
		if err := store.Insert(context.Background(), evt); err != nil {
			return store, fmt.Errorf("make store: [%d] failed to insert event: %w", i, err)
		}
	}
	return store, nil
}

func runQuery[ID goes.ID](s event.Store[ID], q event.QueryOf[ID]) ([]event.Of[any, ID], error) {
	events, _, err := s.Query(context.Background(), q)
	if err != nil {
		return nil, fmt.Errorf("expected store.Query to succeed; got %w", err)
	}
	result, err := streams.Drain(context.Background(), events)
	if err != nil {
		return nil, fmt.Errorf("expected cursor.All to succeed; got %w", err)
	}
	return result, nil
}
