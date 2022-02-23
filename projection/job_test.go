package projection_test

import (
	"context"
	"log"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventstore"
	"github.com/modernice/goes/event/query"
	"github.com/modernice/goes/event/test"
	"github.com/modernice/goes/helper/pick"
	"github.com/modernice/goes/helper/streams"
	"github.com/modernice/goes/internal/projectiontest"
	"github.com/modernice/goes/projection"
)

func TestJob_Events(t *testing.T) {
	ctx := context.Background()
	store, storeEvents := newEventStore(t)

	q := query.New(query.Name("foo", "bar"))

	job := projection.NewJob(ctx, store, q)

	str, errs, err := job.Events(job)
	if err != nil {
		t.Fatalf("Events failed with %q", err)
	}

	events, err := streams.Drain(ctx, str, errs)
	if err != nil {
		t.Fatalf("drain Events: %v", err)
	}

	if len(events) != 2 {
		t.Fatalf("Events should return %d Events; got %d", 2, len(events))
	}

	test.AssertEqualEventsUnsorted(t, events, storeEvents[:2])
}

func TestJob_Events_additionalFilter(t *testing.T) {
	ctx := context.Background()
	store, storeEvents := newEventStore(t)

	q := query.New(query.Name("foo", "bar", "baz"))

	job := projection.NewJob(ctx, store, q)

	str, errs, err := job.Events(job, query.New(query.Name("foo", "bar")), query.New(query.Name("bar")))
	if err != nil {
		t.Fatalf("Events failed with %q", err)
	}

	events, err := streams.Drain(ctx, str, errs)
	if err != nil {
		t.Fatalf("drain Events: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("Events should return %d Event; got %d", 1, len(events))
	}

	if !event.Equal(events[0], storeEvents[1]) {
		t.Fatalf("Events returned wrong Event. want=%v got=%v", storeEvents[1], events[0])
	}
}

func TestJob_EventsOf(t *testing.T) {
	storeEvents := []event.Event{
		event.New[any]("foo", test.FooEventData{}, event.Aggregate(uuid.New(), "foo-agg", 0)),
		event.New[any]("foo", test.FooEventData{}, event.Aggregate(uuid.New(), "bar-agg", 0)),
		event.New[any]("foo", test.FooEventData{}, event.Aggregate(uuid.New(), "baz-agg", 0)),
		event.New[any]("foo", test.FooEventData{}, event.Aggregate(uuid.New(), "foobar-agg", 0)),

		event.New[any]("bar", test.FooEventData{}, event.Aggregate(uuid.New(), "foo-agg", 0)),
		event.New[any]("bar", test.FooEventData{}, event.Aggregate(uuid.New(), "bar-agg", 0)),
		event.New[any]("bar", test.FooEventData{}, event.Aggregate(uuid.New(), "baz-agg", 0)),
		event.New[any]("bar", test.FooEventData{}, event.Aggregate(uuid.New(), "foobar-agg", 0)),
	}

	store, _ := newEventStore(t, storeEvents...)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	job := projection.NewJob(ctx, store, query.New(query.Name("foo")))

	str, errs, err := job.EventsOf(job, "foo-agg", "baz-agg", "foobar-agg")
	if err != nil {
		t.Fatalf("EventsOf failed with %q", err)
	}

	events, err := streams.Drain(ctx, str, errs)
	if err != nil {
		t.Fatalf("drain Events: %v", err)
	}

	test.AssertEqualEventsUnsorted(t, events, []event.Event{
		storeEvents[0], storeEvents[2], storeEvents[3],
	})
}

func TestJob_EventsFor(t *testing.T) {
	ctx := context.Background()
	target := projectiontest.NewMockProjection()
	store, storeEvents := newEventStore(t)

	job := projection.NewJob(ctx, store, query.New(query.Name("foo", "bar", "baz")))

	str, errs, err := job.EventsFor(job, target)
	if err != nil {
		t.Fatalf("EventsFor failed with %q", err)
	}

	events, err := streams.Drain(ctx, str, errs)
	if err != nil {
		t.Fatalf("drain events: %v", err)
	}

	test.AssertEqualEventsUnsorted(t, storeEvents, events)
}

func TestJob_EventsFor_Progressor(t *testing.T) {
	ctx := context.Background()
	target := projectiontest.NewMockProgressor()
	now := time.Now()
	target.SetProgress(now)

	storeEvents := []event.Event{
		event.New[any]("foo", test.FooEventData{}, event.Time(now.Add(-time.Minute))),
		event.New[any]("foo", test.FooEventData{}, event.Time(now)),
		event.New[any]("foo", test.FooEventData{}, event.Time(now.Add(time.Minute))),
		event.New[any]("foo", test.FooEventData{}, event.Time(now.Add(time.Hour))),
	}
	store, _ := newEventStore(t, storeEvents...)

	job := projection.NewJob(ctx, store, query.New())

	str, errs, err := job.EventsFor(job, target)
	if err != nil {
		t.Fatalf("EventsFor failed with %q", err)
	}

	events, err := streams.Drain(ctx, str, errs)
	if err != nil {
		t.Fatalf("drain Events: %v", err)
	}

	test.AssertEqualEventsUnsorted(t, events, storeEvents[2:])
}

func TestJob_Aggregates(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	storeEvents := []event.Event{
		event.New[any]("foo", test.FooEventData{}, event.Aggregate(uuid.New(), "foo-agg", 0), event.Time(now)),
		event.New[any]("foo", test.FooEventData{}, event.Aggregate(uuid.New(), "bar-agg", 0), event.Time(now.Add(time.Second))),
		event.New[any]("foo", test.FooEventData{}, event.Aggregate(uuid.New(), "baz-agg", 0), event.Time(now.Add(2*time.Second))),
		event.New[any]("foo", test.FooEventData{}, event.Aggregate(uuid.New(), "foobar-agg", 0), event.Time(now.Add(3*time.Second))),
	}
	store, _ := newEventStore(t, storeEvents...)

	job := projection.NewJob(ctx, store, query.New(query.SortBy(event.SortTime, event.SortAsc)))

	str, errs, err := job.Aggregates(job)
	if err != nil {
		t.Fatalf("Aggregates failed with %q", err)
	}

	aggregates, err := streams.Drain(ctx, str, errs)
	if err != nil {
		t.Fatalf("DrainTuples failed with %q", err)
	}

	want := make([]aggregate.Ref, len(storeEvents))
	for i, evt := range storeEvents {
		id, name, _ := evt.Aggregate()
		want[i] = aggregate.Ref{Name: name, ID: id}
	}

	if !reflect.DeepEqual(want, aggregates) {
		t.Fatalf("Job returned wrong Aggregates. want=%v got=%v", want, aggregates)
	}
}

func TestJob_Aggregates_specific(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	storeEvents := []event.Event{
		event.New[any]("foo", test.FooEventData{}, event.Aggregate(uuid.New(), "foo-agg", 0), event.Time(now)),
		event.New[any]("foo", test.FooEventData{}, event.Aggregate(uuid.New(), "bar-agg", 0), event.Time(now.Add(time.Second))),
		event.New[any]("foo", test.FooEventData{}, event.Aggregate(uuid.New(), "baz-agg", 0), event.Time(now.Add(2*time.Second))),
		event.New[any]("foo", test.FooEventData{}, event.Aggregate(uuid.New(), "foobar-agg", 0), event.Time(now.Add(3*time.Second))),
	}
	store, _ := newEventStore(t, storeEvents...)

	job := projection.NewJob(ctx, store, query.New(query.SortBy(event.SortTime, event.SortAsc)))

	str, errs, err := job.Aggregates(job, "bar-agg", "foobar-agg")
	if err != nil {
		t.Fatalf("Aggregates failed with %q", err)
	}

	aggregates, err := streams.Drain(ctx, str, errs)
	if err != nil {
		t.Fatalf("DrainTuples failed with %q", err)
	}

	want := []aggregate.Ref{
		{Name: pick.AggregateName(storeEvents[1]), ID: pick.AggregateID(storeEvents[1])},
		{Name: pick.AggregateName(storeEvents[3]), ID: pick.AggregateID(storeEvents[3])},
	}

	if !reflect.DeepEqual(want, aggregates) {
		t.Fatalf("Job returned wrong Aggregates. want=%v got=%v", want, aggregates)
	}
}

func TestJob_Aggregate(t *testing.T) {
	ctx := context.Background()
	storeEvents := []event.Event{
		event.New[any]("foo", test.FooEventData{}, event.Aggregate(uuid.New(), "foo-agg", 0)),
		event.New[any]("foo", test.FooEventData{}, event.Aggregate(uuid.New(), "bar-agg", 0)),
		event.New[any]("foo", test.FooEventData{}, event.Aggregate(uuid.New(), "baz-agg", 0)),
		event.New[any]("foo", test.FooEventData{}, event.Aggregate(uuid.New(), "foobar-agg", 0)),
	}
	store, _ := newEventStore(t, storeEvents...)

	job := projection.NewJob(ctx, store, query.New())

	id, err := job.Aggregate(job, "baz-agg")
	if err != nil {
		t.Fatalf("Aggregate failed with %q", err)
	}

	if id != pick.AggregateID(storeEvents[2]) {
		t.Fatalf("Aggregate should return %q; got %q", pick.AggregateID(storeEvents[2]), id)
	}
}

func TestJob_Apply(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	storeEvents := []event.Event{
		event.New[any]("foo", test.FooEventData{}, event.Aggregate(uuid.New(), "foo-agg", 0), event.Time(now)),
		event.New[any]("foo", test.FooEventData{}, event.Aggregate(uuid.New(), "foo-agg", 0), event.Time(now.Add(time.Second))),
		event.New[any]("foo", test.FooEventData{}, event.Aggregate(uuid.New(), "foo-agg", 0), event.Time(now.Add(time.Minute))),
		event.New[any]("foo", test.FooEventData{}, event.Aggregate(uuid.New(), "bar-agg", 0), event.Time(now.Add(2*time.Minute))),
		event.New[any]("foo", test.FooEventData{}, event.Aggregate(uuid.New(), "bar-agg", 0), event.Time(now.Add(3*time.Minute))),
	}
	store, _ := newEventStore(t, storeEvents...)

	job := projection.NewJob(ctx, store, query.New(
		query.AggregateName("foo-agg"),
		query.SortBy(event.SortTime, event.SortAsc),
	))

	proj := projectiontest.NewMockProjection()

	if err := job.Apply(job, proj); err != nil {
		t.Fatalf("Apply failed with %q", err)
	}

	test.AssertEqualEvents(t, storeEvents[:3], proj.AppliedEvents)
}

func TestJob_Events_cache(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	storeEvents := []event.Event{
		event.New[any]("foo", test.FooEventData{}, event.Aggregate(uuid.New(), "foo-agg", 0), event.Time(now)),
		event.New[any]("foo", test.FooEventData{}, event.Aggregate(uuid.New(), "foo-agg", 0), event.Time(now.Add(time.Second))),
		event.New[any]("foo", test.FooEventData{}, event.Aggregate(uuid.New(), "foo-agg", 0), event.Time(now.Add(time.Minute))),
		event.New[any]("foo", test.FooEventData{}, event.Aggregate(uuid.New(), "bar-agg", 0), event.Time(now.Add(2*time.Minute))),
		event.New[any]("foo", test.FooEventData{}, event.Aggregate(uuid.New(), "bar-agg", 0), event.Time(now.Add(3*time.Minute))),
	}
	store, _ := newEventStore(t, storeEvents...)
	delayedStore := newDelayedEventStore(store, 100*time.Millisecond)

	job := projection.NewJob(ctx, delayedStore, query.New())

	start := time.Now()
	str, errs, err := job.Events(job)
	if err != nil {
		t.Fatalf("Events failed with %q", err)
	}

	events, err := streams.Drain(ctx, str, errs)
	if err != nil {
		t.Fatalf("drain Events: %v", err)
	}

	dur := time.Since(start)
	if dur < 100*time.Millisecond || dur > 200*time.Millisecond {
		t.Fatalf("first query should take ~100ms; took %v", dur)
	}

	test.AssertEqualEventsUnsorted(t, events, storeEvents)

	start = time.Now()
	if str, errs, err = job.Events(job); err != nil {
		t.Fatalf("Events failed with %q", err)
	}

	if events, err = streams.Drain(ctx, str, errs); err != nil {
		t.Fatalf("drain Events: %v", err)
	}

	test.AssertEqualEventsUnsorted(t, events, storeEvents)

	dur = time.Since(start)
	if dur >= 100*time.Millisecond {
		t.Fatalf("subsequent queries should take less than 100ms; took %v", dur)
	}
}

func TestWithFilter(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	storeEvents := []event.Event{
		event.New[any]("foo", test.FooEventData{}, event.Aggregate(uuid.New(), "foo", 0), event.Time(now)),
		event.New[any]("foo", test.FooEventData{}, event.Aggregate(uuid.New(), "bar", 0), event.Time(now.Add(time.Second))),
		event.New[any]("foo", test.FooEventData{}, event.Aggregate(uuid.New(), "baz", 0), event.Time(now.Add(time.Minute))),
		event.New[any]("foo", test.FooEventData{}, event.Aggregate(uuid.New(), "foobar", 0), event.Time(now.Add(2*time.Minute))),
		event.New[any]("foo", test.FooEventData{}, event.Aggregate(uuid.New(), "barbaz", 0), event.Time(now.Add(3*time.Minute))),
		event.New[any]("foo", test.FooEventData{}, event.Aggregate(uuid.New(), "foobaz", 0), event.Time(now.Add(4*time.Minute))),
	}
	store, _ := newEventStore(t, storeEvents...)

	job := projection.NewJob(ctx, store, query.New(query.SortBy(event.SortTime, event.SortAsc)), projection.WithFilter(
		query.New(query.AggregateName("foo", "baz", "barbaz", "foobaz")),
		query.New(query.AggregateName("foo", "barbaz", "foobaz")),
	))

	str, errs, err := job.Events(job)
	if err != nil {
		t.Fatalf("Events failed with %q", err)
	}

	events, err := streams.Drain(ctx, str, errs)
	if err != nil {
		t.Fatalf("drain Events: %v", err)
	}

	test.AssertEqualEvents(t, events, []event.Event{storeEvents[0], storeEvents[4], storeEvents[5]})
}

func TestWithReset(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	storeEvents := []event.Event{
		event.New[any]("foo", test.FooEventData{}, event.Aggregate(uuid.New(), "foo", 0), event.Time(now.Add(-time.Minute))),
		event.New[any]("foo", test.FooEventData{}, event.Aggregate(uuid.New(), "foo", 0), event.Time(now)),
		event.New[any]("foo", test.FooEventData{}, event.Aggregate(uuid.New(), "bar", 0), event.Time(now.Add(time.Second))),
		event.New[any]("foo", test.FooEventData{}, event.Aggregate(uuid.New(), "baz", 0), event.Time(now.Add(time.Minute))),
		event.New[any]("foo", test.FooEventData{}, event.Aggregate(uuid.New(), "foobar", 0), event.Time(now.Add(2*time.Minute))),
		event.New[any]("foo", test.FooEventData{}, event.Aggregate(uuid.New(), "barbaz", 0), event.Time(now.Add(3*time.Minute))),
		event.New[any]("foo", test.FooEventData{}, event.Aggregate(uuid.New(), "foobaz", 0), event.Time(now.Add(4*time.Minute))),
	}
	store, _ := newEventStore(t, storeEvents...)

	proj := projectiontest.NewMockResetProjection(3)

	if err := projection.Apply(proj, storeEvents); err != nil {
		t.Fatalf("Apply failed with %q", err)
	}

	job := projection.NewJob(ctx, store, query.New(query.SortBy(event.SortTime, event.SortAsc)), projection.WithReset())

	if err := job.Apply(job, proj); err != nil {
		t.Fatalf("Apply failed with %q", err)
	}

	test.AssertEqualEvents(t, proj.AppliedEvents, storeEvents)

	got := proj.Progress()
	want := storeEvents[6].Time()
	if !got.Equal(want) {
		log.Printf("\n%#v\n\n%#v", want, got)
		t.Fatalf("Progress should be %v; is %v", want, got)
	}

	if proj.Foo != 0 {
		t.Fatalf("Projection should have been reset")
	}
}

func newEventStore(t *testing.T, events ...event.Event) (event.Store, []event.Event) {
	store := eventstore.New()
	now := time.Now()
	if len(events) == 0 {
		events = []event.Event{
			event.New[any]("foo", test.FooEventData{}, event.Time(now)),
			event.New[any]("bar", test.FooEventData{}, event.Time(now.Add(time.Second))),
			event.New[any]("baz", test.FooEventData{}, event.Time(now.Add(time.Minute))),
		}
	}
	if err := store.Insert(context.Background(), events...); err != nil {
		t.Fatalf("insert Events: %v", err)
	}
	return store, events
}

type delayedEventStore struct {
	event.Store
	delay time.Duration
}

func newDelayedEventStore(store event.Store, delay time.Duration) *delayedEventStore {
	return &delayedEventStore{Store: store, delay: delay}
}

func (s *delayedEventStore) Query(ctx context.Context, q event.Query) (<-chan event.Event, <-chan error, error) {
	timer := time.NewTimer(s.delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	case <-timer.C:
	}

	return s.Store.Query(ctx, q)
}
