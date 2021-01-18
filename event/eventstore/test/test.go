package test

import (
	"context"
	"fmt"
	"testing"
	stdtime "time"

	"github.com/google/uuid"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/cursor"
	"github.com/modernice/goes/event/query"
	"github.com/modernice/goes/event/query/time"
	"github.com/modernice/goes/event/query/version"
	"github.com/modernice/goes/event/test"
	"golang.org/x/sync/errgroup"
)

// EventStoreFactory creates an event.Store.
type EventStoreFactory func() event.Store

// EventStore tests an event.Store implementation.
func EventStore(t *testing.T, newStore EventStoreFactory) {
	testInsert(t, newStore)
	testFind(t, newStore)
	testDelete(t, newStore)
	testConcurrentStore(t, newStore)
	testQuery(t, newStore)
}

func testInsert(t *testing.T, newStore EventStoreFactory) {
	store := newStore()

	// inserting an event shouldn't fail
	evt := event.New("foo", test.FooEventData{A: "foo"}, event.Aggregate("bar", uuid.New(), 3))
	if err := store.Insert(context.Background(), evt); err != nil {
		t.Errorf("inserting an event shouldn't fail: %v", err)
	}

	// inserting an event with an existing id should fail
	evt = event.New("foo", test.FooEventData{A: "bar"}, event.ID(evt.ID()))
	if err := store.Insert(context.Background(), evt); err == nil {
		t.Errorf("inserting an event with an existing id should fail; err=%v", err)
	}
}

func testFind(t *testing.T, newStore EventStoreFactory) {
	store := newStore()

	found, err := store.Find(context.Background(), uuid.New())
	if err == nil {
		t.Errorf("expected store.Find to return an error; got %#v", err)
	}
	if found != nil {
		t.Errorf("expected store.Find to return no event; got %#v", found)
	}

	evt := event.New("foo", test.FooEventData{A: "foo"})
	if err := store.Insert(context.Background(), evt); err != nil {
		t.Fatal(fmt.Errorf("store.Insert failed: %w", err))
	}

	found, err = store.Find(context.Background(), evt.ID())
	if err != nil {
		t.Errorf("expected store.Find not to return error; got %#v", err)
	}
	if !event.Equal(found, evt) {
		t.Errorf("found event doesn't match inserted event\ninserted: %#v\n\nfound: %#v", evt, found)
	}
}

func testDelete(t *testing.T, newStore EventStoreFactory) {
	store := newStore()

	foo := event.New("foo", test.FooEventData{A: "foo"})
	bar := event.New("bar", test.BarEventData{A: "bar"})

	if err := store.Insert(context.Background(), foo); err != nil {
		t.Fatal(fmt.Errorf("%q: store.Insert failed: %w", "foo", err))
	}

	if err := store.Insert(context.Background(), bar); err != nil {
		t.Fatal(fmt.Errorf("%q: store.Insert failed: %w", "bar", err))
	}

	if err := store.Delete(context.Background(), foo); err != nil {
		t.Fatal(fmt.Errorf("%q: expected store.Delete not to return an error; got %#v", "foo", err))
	}

	found, err := store.Find(context.Background(), foo.ID())
	if err == nil {
		t.Error(fmt.Errorf("%q: expected store.Find to return an error; got %#v", "foo", err))
	}
	if found != nil {
		t.Errorf("%q: expected store.Find not to return an event; got %#v", "foo", found)
	}

	found, err = store.Find(context.Background(), bar.ID())
	if err != nil {
		t.Error(fmt.Errorf("%q: expected store.Find not to return an error; got %#v", "bar", err))
	}
	if !event.Equal(found, bar) {
		t.Errorf("%q: found event doesn't match inserted event\ninserted: %#v\n\nfound: %#v", "bar", bar, found)
	}
}

func testConcurrentStore(t *testing.T, newStore EventStoreFactory) {
	testConcurrentInsert(t, newStore)
	testConcurrentFind(t, newStore)
	testConcurrentDelete(t, newStore)
}

func testConcurrentInsert(t *testing.T, newStore EventStoreFactory) {
	store := newStore()
	group, ctx := errgroup.WithContext(context.Background())
	for i := 0; i < 30; i++ {
		i := i
		group.Go(func() error {
			evt := event.New("foo", test.FooEventData{A: "foo"})
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
	store := newStore()
	evt := event.New("foo", test.FooEventData{A: "foo"})
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
	store := newStore()

	events := make([]event.Event, 30)
	for i := range events {
		events[i] = event.New("foo", test.FooEventData{A: "foo"})
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
	testQueryName(t, newStore)
	testQueryID(t, newStore)
	testQueryTime(t, newStore)
	testQueryAggregateName(t, newStore)
	testQueryAggregateID(t, newStore)
	testQueryAggregateVersion(t, newStore)
}

func testQueryName(t *testing.T, newStore EventStoreFactory) {
	events := []event.Event{
		event.New("foo", test.FooEventData{A: "foo"}),
		event.New("foo", test.FooEventData{A: "foo"}, event.Aggregate("foo", uuid.New(), 10)),
		event.New("foo", test.FooEventData{A: "foo"}, event.Aggregate("foo", uuid.New(), 20)),
	}

	// given a store with 3 "foo" events
	store, err := makeStore(newStore, events...)
	if err != nil {
		t.Fatal(err)
	}

	// querying "foo" events should return all 3 events
	cur, err := store.Query(context.Background(), query.New(query.Name("foo")))
	if err != nil {
		t.Fatal(fmt.Errorf("expected store.Query not to return an error; got %#v", err))
	}

	result, err := cursor.All(context.Background(), cur)
	if err != nil {
		t.Fatal(fmt.Errorf("expected cursor.All not to return an error; got %#v", err))
	}

	if !test.EqualEvents(events, result) {
		t.Fatalf("expected cursor events to match original events\noriginal: %#v\n\ngot: %#v", events, result)
	}
}

func testQueryID(t *testing.T, newStore EventStoreFactory) {
	events := []event.Event{
		event.New("foo", test.FooEventData{A: "foo"}, event.ID(uuid.New())),
		event.New("bar", test.BarEventData{A: "bar"}, event.ID(uuid.New())),
		event.New("baz", test.BazEventData{A: "baz"}, event.ID(uuid.New())),
	}

	store, err := makeStore(newStore, events...)
	if err != nil {
		t.Fatal(err)
	}

	cur, err := store.Query(context.Background(), query.New(query.ID(
		events[0].ID(),
		events[2].ID(),
	)))
	if err != nil {
		t.Fatal(fmt.Errorf("expected store.Query not to return an error; got %#v", err))
	}

	result, err := cursor.All(context.Background(), cur)
	if err != nil {
		t.Fatal(fmt.Errorf("expected cursor.All not to return an error; got %#v", err))
	}

	want := []event.Event{events[0], events[2]}
	if !test.EqualEvents(want, result) {
		t.Fatalf("cursor returned the wrong events\nwant: %#v\n\ngot: %#v", want, result)
	}
}

func testQueryTime(t *testing.T, newStore EventStoreFactory) {
	now := stdtime.Now()
	events := []event.Event{
		event.New("foo", test.FooEventData{A: "foo"}, event.Time(now)),
		event.New("bar", test.BarEventData{A: "bar"}, event.Time(now.AddDate(0, 1, 0))),
		event.New("baz", test.BazEventData{A: "baz"}, event.Time(now.AddDate(1, 0, 0))),
	}

	store, err := makeStore(newStore, events...)
	if err != nil {
		t.Fatal(err)
	}

	cur, err := store.Query(context.Background(), query.New(query.Time(
		time.Min(events[1].Time()),
		time.Max(events[2].Time()),
	)))
	if err != nil {
		t.Fatal(fmt.Errorf("expected store.Query not to return an error; got %#v", err))
	}

	result, err := cursor.All(context.Background(), cur)
	if err != nil {
		t.Fatal(fmt.Errorf("expected cursor.All not to return an error; got %#v", err))
	}

	want := events[1:]
	if !test.EqualEvents(want, result) {
		t.Fatalf("cursor returned the wrong events\nwant: %#v\n\ngot: %#v", want, result)
	}
}

func testQueryAggregateName(t *testing.T, newStore EventStoreFactory) {
	events := []event.Event{
		event.New("foo", test.FooEventData{A: "foo"}),
		event.New("foo", test.FooEventData{A: "foo"}, event.Aggregate("foo", uuid.New(), 10)),
		event.New("foo", test.FooEventData{A: "foo"}, event.Aggregate("foo", uuid.New(), 20)),
	}

	store, err := makeStore(newStore, events...)
	if err != nil {
		t.Fatal(err)
	}

	cur, err := store.Query(context.Background(), query.New(query.AggregateName("foo")))
	if err != nil {
		t.Fatal(fmt.Errorf("expected store.Query not to return an error; got %#v", err))
	}

	result, err := cursor.All(context.Background(), cur)
	if err != nil {
		t.Fatal(fmt.Errorf("expected cursor.All not to return an error; got %#v", err))
	}

	want := events[1:]
	if !test.EqualEvents(want, result) {
		t.Fatalf("expected cursor events to match original events\noriginal: %#v\n\ngot: %#v", want, result)
	}
}

func testQueryAggregateID(t *testing.T, newStore EventStoreFactory) {
	events := []event.Event{
		event.New("foo", test.FooEventData{A: "foo"}, event.Aggregate("foo", uuid.New(), 5)),
		event.New("foo", test.FooEventData{A: "foo"}, event.Aggregate("foo", uuid.New(), 10)),
		event.New("foo", test.FooEventData{A: "foo"}, event.Aggregate("foo", uuid.New(), 20)),
	}

	store, err := makeStore(newStore, events...)
	if err != nil {
		t.Fatal(err)
	}

	cur, err := store.Query(context.Background(), query.New(query.AggregateID(
		events[0].AggregateID(),
		events[2].AggregateID(),
	)))
	if err != nil {
		t.Fatal(fmt.Errorf("expected store.Query not to return an error; got %#v", err))
	}

	result, err := cursor.All(context.Background(), cur)
	if err != nil {
		t.Fatal(fmt.Errorf("expected cursor.All not to return an error; got %#v", err))
	}

	want := []event.Event{events[0], events[2]}
	if !test.EqualEvents(want, result) {
		t.Fatalf("cursor returned the wrong events\nwant: %#v\n\ngot: %#v", want, result)
	}
}

func testQueryAggregateVersion(t *testing.T, newStore EventStoreFactory) {
	events := []event.Event{
		event.New("foo", test.FooEventData{A: "foo"}, event.Aggregate("foo", uuid.New(), 2)),
		event.New("foo", test.FooEventData{A: "foo"}, event.Aggregate("foo", uuid.New(), 4)),
		event.New("foo", test.FooEventData{A: "foo"}, event.Aggregate("foo", uuid.New(), 8)),
		event.New("foo", test.FooEventData{A: "foo"}, event.Aggregate("foo", uuid.New(), 16)),
		event.New("foo", test.FooEventData{A: "foo"}, event.Aggregate("foo", uuid.New(), 32)),
	}

	store, err := makeStore(newStore, events...)
	if err != nil {
		t.Fatal(err)
	}

	cur, err := store.Query(context.Background(), query.New(query.AggregateVersion(
		version.Min(5),
		version.Max(16),
	)))
	if err != nil {
		t.Fatal(fmt.Errorf("expected store.Query not to return an error; got %#v", err))
	}

	result, err := cursor.All(context.Background(), cur)
	if err != nil {
		t.Fatal(fmt.Errorf("expected cursor.All not to return an error; got %#v", err))
	}

	want := events[2:4]
	if !test.EqualEvents(want, result) {
		t.Fatalf("cursor returned the wrong events\nwant: %#v\n\ngot: %#v", want, result)
	}
}

func makeStore(newStore EventStoreFactory, events ...event.Event) (event.Store, error) {
	store := newStore()
	for i, evt := range events {
		if err := store.Insert(context.Background(), evt); err != nil {
			return store, fmt.Errorf("make store: [%d] failed to insert event: %w", i, err)
		}
	}
	return store, nil
}
