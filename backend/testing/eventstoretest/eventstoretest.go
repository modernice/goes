package eventstoretest

import (
	"context"
	"fmt"
	"testing"
	stdtime "time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
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

// EventStoreFactory creates an event.Store.
type EventStoreFactory func(codec.Encoding) event.Store

// Run tests an event store implementation.
func Run(t *testing.T, name string, newStore EventStoreFactory) {
	t.Run(name, func(t *testing.T) {
		run(t, "Insert", newStore, testInsert)
		run(t, "Find", newStore, testFind)
		run(t, "Delete", newStore, testDelete)
		run(t, "Concurrency", newStore, testConcurrency)
		run(t, "Query", newStore, testQuery)
	})
}

func run(t *testing.T, name string, newStore EventStoreFactory, runner func(*testing.T, EventStoreFactory)) {
	t.Run(name, func(t *testing.T) {
		runner(t, newStore)
	})
}

func testInsert(t *testing.T, newStore EventStoreFactory) {
	run(t, "SingleInsert", newStore, testSingleInsert)
	run(t, "MultiInsert", newStore, testMultiInsert)
	run(t, "InvalidMultiInsert", newStore, testInvalidMultiInsert)
}

func testSingleInsert(t *testing.T, newStore EventStoreFactory) {
	store := newStore(test.NewEncoder())

	// inserting an event shouldn't fail
	evt := event.New[any]("foo", test.FooEventData{A: "foo"}, event.Aggregate[any](uuid.New(), "bar", 3))
	if err := store.Insert(context.Background(), evt); err != nil {
		t.Fatalf("inserting an event shouldn't fail: %v", err)
	}

	// inserting an event with an existing id should fail
	evt = event.New[any]("foo", test.FooEventData{A: "bar"}, event.ID[any](evt.ID()))
	if err := store.Insert(context.Background(), evt); err == nil {
		t.Errorf("inserting an event with an existing id should fail; err=%v", err)
	}
}

func testMultiInsert(t *testing.T, newStore EventStoreFactory) {
	store := newStore(test.NewEncoder())
	events := []event.Of[any]{
		event.New[any]("foo", test.FooEventData{A: "foo"}),
		event.New[any]("foo", test.FooEventData{A: "foo"}),
		event.New[any]("foo", test.FooEventData{A: "foo"}),
	}

	if err := store.Insert(context.Background(), events...); err != nil {
		t.Fatalf("expected store.Insert to succeed; got %#v", err)
	}

	result, err := runQuery(store, query.New())
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
func testInvalidMultiInsert(t *testing.T, newStore EventStoreFactory) {
	store := newStore(test.NewEncoder())
	aggregateID := uuid.New()
	eventID := uuid.New()
	events := []event.Of[any]{
		event.New[any]("foo", test.FooEventData{A: "foo"}, event.Aggregate[any](aggregateID, "foo", 0)),
		event.New[any]("foo", test.FooEventData{A: "foo"}, event.Aggregate[any](aggregateID, "foo", 1)),
		event.New[any]("foo", test.FooEventData{A: "foo"}, event.Aggregate[any](aggregateID, "foo", 2)),
		event.New[any]("foo", test.FooEventData{A: "foo"}, event.Aggregate[any](aggregateID, "foo", 2)),
		event.New[any]("foo", test.FooEventData{A: "foo"}, event.Aggregate[any](aggregateID, "foo", 3), event.ID[any](eventID)),
		event.New[any]("foo", test.FooEventData{A: "foo"}, event.Aggregate[any](aggregateID, "foo", 4), event.ID[any](eventID)),
	}

	if err := store.Insert(context.Background(), events...); err == nil {
		t.Fatalf("expected store.Insert to fail; got %#v", err)
	}
}

func testFind(t *testing.T, newStore EventStoreFactory) {
	store := newStore(test.NewEncoder())

	found, err := store.Find(context.Background(), uuid.New())
	if err == nil {
		t.Errorf("expected store.Find to return an error; got %#v", err)
	}
	if found != nil {
		t.Errorf("expected store.Find to return no event; got %#v", found)
	}

	evt := event.New[any]("foo", test.FooEventData{A: "foo"})
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

func testDelete(t *testing.T, newStore EventStoreFactory) {
	store := newStore(test.NewEncoder())

	foo := event.New[any]("foo", test.FooEventData{A: "foo"})
	bar := event.New[any]("bar", test.BarEventData{A: "bar"})

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

func testConcurrency(t *testing.T, newStore EventStoreFactory) {
	run(t, "ConcurrentInsert", newStore, testConcurrentInsert)
	run(t, "ConcurrentFind", newStore, testConcurrentFind)
	run(t, "ConcurrentDelete", newStore, testConcurrentDelete)
}

func testConcurrentInsert(t *testing.T, newStore EventStoreFactory) {
	store := newStore(test.NewEncoder())
	group, ctx := errgroup.WithContext(context.Background())
	for i := 0; i < 30; i++ {
		i := i
		group.Go(func() error {
			evt := event.New[any]("foo", test.FooEventData{A: "foo"})
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

func testConcurrentFind(t *testing.T, newStore EventStoreFactory) {
	store := newStore(test.NewEncoder())
	evt := event.New[any]("foo", test.FooEventData{A: "foo"})
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

func testConcurrentDelete(t *testing.T, newStore EventStoreFactory) {
	store := newStore(test.NewEncoder())

	events := make([]event.Of[any], 30)
	for i := range events {
		events[i] = event.New[any]("foo", test.FooEventData{A: "foo"})
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

func testQuery(t *testing.T, newStore EventStoreFactory) {
	run(t, "QueryName", newStore, testQueryName)
	run(t, "QueryID", newStore, testQueryID)
	run(t, "QueryTime", newStore, testQueryTime)
	run(t, "QueryAggregateName", newStore, testQueryAggregateName)
	run(t, "QueryAggregateID", newStore, testQueryAggregateID)
	run(t, "QueryAggregateVersion", newStore, testQueryAggregateVersion)
	run(t, "QueryAggregate", newStore, testQueryAggregate)
	run(t, "Sorting", newStore, testQuerySorting)
}

func testQueryName(t *testing.T, newStore EventStoreFactory) {
	events := []event.Of[any]{
		event.New[any]("foo", test.FooEventData{A: "foo"}),
		event.New[any]("foo", test.FooEventData{A: "foo"}, event.Aggregate[any](uuid.New(), "foo", 10)),
		event.New[any]("foo", test.FooEventData{A: "foo"}, event.Aggregate[any](uuid.New(), "foo", 20)),
	}

	// given a store with 3 "foo" events
	store, err := makeStore(newStore, events...)
	if err != nil {
		t.Fatal(err)
	}

	// querying "foo" events should return all 3 events
	result, err := runQuery(store, query.New(query.Name("foo")))
	if err != nil {
		t.Fatal(err)
	}

	test.AssertEqualEventsUnsorted(t, events, result)
}

func testQueryID(t *testing.T, newStore EventStoreFactory) {
	events := []event.Of[any]{
		event.New[any]("foo", test.FooEventData{A: "foo"}, event.ID[any](uuid.New())),
		event.New[any]("bar", test.BarEventData{A: "bar"}, event.ID[any](uuid.New())),
		event.New[any]("baz", test.BazEventData{A: "baz"}, event.ID[any](uuid.New())),
	}

	store, err := makeStore(newStore, events...)
	if err != nil {
		t.Fatal(err)
	}

	result, err := runQuery(store, query.New(query.ID(
		events[0].ID(),
		events[2].ID(),
	)))
	if err != nil {
		t.Fatal(err)
	}

	want := []event.Of[any]{events[0], events[2]}
	test.AssertEqualEventsUnsorted(t, want, result)
}

func testQueryTime(t *testing.T, newStore EventStoreFactory) {
	now := xtime.Now()
	events := []event.Of[any]{
		event.New[any]("foo", test.FooEventData{A: "foo"}, event.Time[any](now)),
		event.New[any]("bar", test.BarEventData{A: "bar"}, event.Time[any](now.AddDate(0, 1, 0))),
		event.New[any]("baz", test.BazEventData{A: "baz"}, event.Time[any](now.AddDate(1, 0, 0))),
	}

	store, err := makeStore(newStore, events...)
	if err != nil {
		t.Fatal(err)
	}

	result, err := runQuery(store, query.New(query.Time(
		time.Min(events[1].Time()),
		time.Max(events[2].Time()),
	)))
	if err != nil {
		t.Fatal(err)
	}

	want := events[1:]
	test.AssertEqualEventsUnsorted(t, want, result)
}

func testQueryAggregateName(t *testing.T, newStore EventStoreFactory) {
	events := []event.Of[any]{
		event.New[any]("foo", test.FooEventData{A: "foo"}),
		event.New[any]("foo", test.FooEventData{A: "foo"}, event.Aggregate[any](uuid.New(), "foo", 10)),
		event.New[any]("foo", test.FooEventData{A: "foo"}, event.Aggregate[any](uuid.New(), "foo", 20)),
	}

	store, err := makeStore(newStore, events...)
	if err != nil {
		t.Fatal(err)
	}

	result, err := runQuery(store, query.New(query.AggregateName("foo")))
	if err != nil {
		t.Fatal(err)
	}

	want := events[1:]
	test.AssertEqualEventsUnsorted(t, want, result)
}

func testQueryAggregateID(t *testing.T, newStore EventStoreFactory) {
	events := []event.Of[any]{
		event.New[any]("foo", test.FooEventData{A: "foo"}, event.Aggregate[any](uuid.New(), "foo", 5)),
		event.New[any]("foo", test.FooEventData{A: "foo"}, event.Aggregate[any](uuid.New(), "foo", 10)),
		event.New[any]("foo", test.FooEventData{A: "foo"}, event.Aggregate[any](uuid.New(), "foo", 20)),
	}

	store, err := makeStore(newStore, events...)
	if err != nil {
		t.Fatal(err)
	}

	result, err := runQuery(store, query.New(query.AggregateID(
		pick.AggregateID(events[0]),
		pick.AggregateID(events[2]),
	)))
	if err != nil {
		t.Fatal(err)
	}

	want := []event.Of[any]{events[0], events[2]}
	test.AssertEqualEventsUnsorted(t, want, result)
}

func testQueryAggregateVersion(t *testing.T, newStore EventStoreFactory) {
	events := []event.Of[any]{
		event.New[any]("foo", test.FooEventData{A: "foo"}, event.Aggregate[any](uuid.New(), "foo", 2)),
		event.New[any]("foo", test.FooEventData{A: "foo"}, event.Aggregate[any](uuid.New(), "foo", 4)),
		event.New[any]("foo", test.FooEventData{A: "foo"}, event.Aggregate[any](uuid.New(), "foo", 8)),
		event.New[any]("foo", test.FooEventData{A: "foo"}, event.Aggregate[any](uuid.New(), "foo", 16)),
		event.New[any]("foo", test.FooEventData{A: "foo"}, event.Aggregate[any](uuid.New(), "foo", 32)),
	}

	store, err := makeStore(newStore, events...)
	if err != nil {
		t.Fatal(err)
	}

	result, err := runQuery(store, query.New(query.AggregateVersion(
		version.Min(5),
		version.Max(16),
	)))
	if err != nil {
		t.Fatal(err)
	}

	want := events[2:4]
	test.AssertEqualEventsUnsorted(t, want, result)
}

func testQueryAggregate(t *testing.T, newStore EventStoreFactory) {
	id := uuid.New()
	events := []event.Of[any]{
		event.New[any]("foo", test.FooEventData{A: "foo"}, event.Aggregate[any](uuid.New(), "foo", 1)),
		event.New[any]("foo", test.FooEventData{A: "foo"}, event.Aggregate[any](id, "foo", 1)),
		event.New[any]("foo", test.FooEventData{A: "foo"}, event.Aggregate[any](uuid.New(), "bar", 1)),
		event.New[any]("foo", test.FooEventData{A: "foo"}, event.Aggregate[any](id, "bar", 1)),
		event.New[any]("foo", test.FooEventData{A: "foo"}, event.Aggregate[any](uuid.New(), "baz", 1)),
		event.New[any]("foo", test.FooEventData{A: "foo"}, event.Aggregate[any](id, "baz", 1)),
	}

	store, err := makeStore(newStore, events...)
	if err != nil {
		t.Fatal(err)
	}

	result, err := runQuery(store, query.New(query.Aggregate("foo", id), query.Aggregate("bar", id)))
	if err != nil {
		t.Fatal(err)
	}

	want := []event.Of[any]{events[1], events[3]}
	test.AssertEqualEventsUnsorted(t, result, want)

	result, err = runQuery(store, query.New(query.Aggregate("foo", id), query.Aggregate("baz", uuid.Nil)))
	if err != nil {
		t.Fatal(err)
	}

	want = []event.Of[any]{events[1], events[4], events[5]}
	test.AssertEqualEventsUnsorted(t, result, want)
}

func testQuerySorting(t *testing.T, newStore EventStoreFactory) {
	now := xtime.Now()
	events := []event.Of[any]{
		event.New[any]("foo", test.FooEventData{A: "foo"}, event.Time[any](now.Add(12*stdtime.Hour)), event.Aggregate[any](uuid.New(), "foo1", 3)),
		event.New[any]("foo", test.FooEventData{A: "foo"}, event.Time[any](now.Add(stdtime.Hour)), event.Aggregate[any](uuid.New(), "foo2", 2)),
		event.New[any]("foo", test.FooEventData{A: "foo"}, event.Time[any](now.Add(48*stdtime.Hour)), event.Aggregate[any](uuid.New(), "foo3", 5)),
		event.New[any]("foo", test.FooEventData{A: "foo"}, event.Time[any](now), event.Aggregate[any](uuid.New(), "foo4", 1)),
		event.New[any]("foo", test.FooEventData{A: "foo"}, event.Time[any](now.Add(24*stdtime.Hour)), event.Aggregate[any](uuid.New(), "foo5", 4)),
	}

	tests := []struct {
		name string
		q    query.Query
		want []event.Of[any]
	}{
		{
			name: "SortTime(asc)",
			q:    query.New(query.SortBy(event.SortTime, event.SortAsc)),
			want: event.Sort(events, event.SortTime, event.SortAsc),
		},
		{
			name: "SortTime(desc)",
			q:    query.New(query.SortBy(event.SortTime, event.SortDesc)),
			want: event.Sort(events, event.SortTime, event.SortDesc),
		},
		{
			name: "Time+SortTime(asc)",
			q: query.New(
				query.Time(
					time.Min(events[0].Time()),
					time.Max(events[4].Time()),
				),
				query.SortBy(event.SortTime, event.SortAsc),
			),
			want: event.Sort([]event.Of[any]{events[0], events[4]}, event.SortTime, event.SortAsc),
		},
		{
			name: "Time+SortTime(desc)",
			q: query.New(
				query.Time(
					time.Min(events[0].Time()),
					time.Max(events[4].Time()),
				),
				query.SortBy(event.SortTime, event.SortDesc),
			),
			want: event.Sort([]event.Of[any]{events[0], events[4]}, event.SortTime, event.SortDesc),
		},
		{
			name: "SortAggregateName(asc)",
			q:    query.New(query.SortBy(event.SortAggregateName, event.SortAsc)),
			want: event.Sort(events, event.SortAggregateName, event.SortAsc),
		},
		{
			name: "SortAggregateName(desc)",
			q:    query.New(query.SortBy(event.SortAggregateName, event.SortDesc)),
			want: event.Sort(events, event.SortAggregateName, event.SortDesc),
		},
		{
			name: "SortAggregateID(asc)",
			q:    query.New(query.SortBy(event.SortAggregateID, event.SortAsc)),
			want: event.Sort(events, event.SortAggregateID, event.SortAsc),
		},
		{
			name: "SortAggregateID(desc)",
			q:    query.New(query.SortBy(event.SortAggregateID, event.SortDesc)),
			want: event.Sort(events, event.SortAggregateID, event.SortDesc),
		},
		{
			name: "SortAggregateVersion(asc)",
			q:    query.New(query.SortBy(event.SortAggregateVersion, event.SortAsc)),
			want: event.Sort(events, event.SortAggregateVersion, event.SortAsc),
		},
		{
			name: "SortAggregateVersion(desc)",
			q:    query.New(query.SortBy(event.SortAggregateVersion, event.SortDesc)),
			want: event.Sort(events, event.SortAggregateVersion, event.SortDesc),
		},
		{
			name: "SortAggregateName(desc)+SortAggregateVersion(asc)",
			q: query.New(
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

			result, err := runQuery(store, tt.q)
			if err != nil {
				t.Fatalf("expected query to succeed: %#v", err)
			}

			test.AssertEqualEvents(t, tt.want, result)
		})
	}
}

func makeStore(newStore EventStoreFactory, events ...event.Of[any]) (event.Store, error) {
	store := newStore(test.NewEncoder())
	for i, evt := range events {
		if err := store.Insert(context.Background(), evt); err != nil {
			return store, fmt.Errorf("make store: [%d] failed to insert event: %w", i, err)
		}
	}
	return store, nil
}

func runQuery(s event.Store, q event.Query) ([]event.Of[any], error) {
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
