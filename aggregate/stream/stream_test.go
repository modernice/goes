package stream_test

import (
	"context"
	"errors"
	"math/rand"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/consistency"
	"github.com/modernice/goes/aggregate/cursor"
	"github.com/modernice/goes/aggregate/stream"
	"github.com/modernice/goes/aggregate/test"
	"github.com/modernice/goes/event"
	ecursor "github.com/modernice/goes/event/cursor"
	etest "github.com/modernice/goes/event/test"
	"github.com/modernice/goes/internal/xevent/xcursor"
)

func TestStream_singleAggregate_sorted(t *testing.T) {
	aggregateName := "foo"
	aggregateID := uuid.New()
	events := []event.Event{
		event.New("foo", etest.FooEventData{}, event.Aggregate(aggregateName, aggregateID, 1)),
		event.New("foo", etest.FooEventData{}, event.Aggregate(aggregateName, aggregateID, 2)),
		event.New("foo", etest.FooEventData{}, event.Aggregate(aggregateName, aggregateID, 3)),
	}
	testStream_singleAggregate(t, events)
}

func TestStream_singleAggregate_unsorted(t *testing.T) {
	aggregateName := "foo"
	aggregateID := uuid.New()
	events := []event.Event{
		event.New("foo", etest.FooEventData{}, event.Aggregate(aggregateName, aggregateID, 3)),
		event.New("foo", etest.FooEventData{}, event.Aggregate(aggregateName, aggregateID, 1)),
		event.New("foo", etest.FooEventData{}, event.Aggregate(aggregateName, aggregateID, 2)),
	}
	testStream_singleAggregate(t, events)
}

func testStream_singleAggregate(t *testing.T, events []event.Event) {
	ecur := ecursor.New(events...)

	appliedEventsC := make(chan event.Event, len(events))
	var wg sync.WaitGroup
	wg.Add(len(events))
	cur := stream.New(
		context.Background(),
		ecur,
		stream.AggregateFactory("foo", func(id uuid.UUID) aggregate.Aggregate {
			return test.NewFoo(id, test.ApplyEventFunc("foo", func(evt event.Event) {
				defer wg.Done()
				appliedEventsC <- evt
			}))
		}),
	)

	wg.Wait()
	close(appliedEventsC)

	as, err := cursor.All(context.Background(), cur)
	if err != nil {
		t.Fatalf("cursor.All should not fail: %v", err)
	}

	if len(as) != 1 {
		t.Errorf("expected stream to return 1 aggregate; got %d", len(as))
	}

	appliedEvents := drainEvents(appliedEventsC)

	etest.AssertEqualEvents(t, event.Sort(events, event.SortAggregateVersion, event.SortAsc), appliedEvents)
}

func TestStream_singleAggregate_inconsistent(t *testing.T) {
	aggregates, getAppliedEvents := makeAggregates(1)
	events, _ := makeEventsFor(aggregates, 10, skipVersion(5))
	fifthEvent := events[4]
	events = shuffleEvents(events)

	ecur := ecursor.New(events...)
	cur := stream.New(
		context.Background(),
		ecur,
		stream.AggregateFactory("foo", func(id uuid.UUID) aggregate.Aggregate {
			return aggregates[0]
		}),
	)

	as, err := cursor.All(context.Background(), cur)

	var cerr *consistency.Error
	if !errors.As(err, &cerr) {
		t.Errorf("expeceted cursor.All to return a %T; got %T", cerr, err)
	}

	if cerr.Kind != consistency.Version {
		t.Errorf("expected cerr.Kind to be %s; got %s", consistency.Version, cerr.Kind)
	}

	if cerr.Event() != fifthEvent {
		t.Errorf("expected cerr.Event to return %#v; got %#v", fifthEvent, cerr.Event())
	}

	if l := len(getAppliedEvents(aggregates[0].AggregateID())); l != 0 {
		t.Errorf("no events should be applied; got %d", l)
	}

	if l := len(as); l != 0 {
		t.Errorf("stream should return no aggregates; got %d:\n\n%#v\n\n", l, as)
	}
}

func TestStream_multipleAggregates(t *testing.T) {
	aggregates, getAppliedEvents := makeAggregates(3)
	aggregateMap := mapAggregates(aggregates)
	events, getEvents := makeEventsFor(aggregates, 5)

	ecur := ecursor.New(events...)
	cur := stream.New(
		context.Background(),
		ecur,
		stream.AggregateFactory("foo", func(id uuid.UUID) aggregate.Aggregate {
			return aggregateMap[id]
		}),
	)

	as, err := cursor.All(context.Background(), cur)
	if err != nil {
		t.Fatalf("cursor.All should not fail: %v", err)
	}

	if len(as) != 3 {
		t.Errorf("expected stream to return %d aggregates; got %d", 3, len(as))
	}

	if !reflect.DeepEqual(
		aggregate.Sort(aggregates, aggregate.SortID, aggregate.SortAsc),
		aggregate.Sort(as, aggregate.SortID, aggregate.SortAsc),
	) {
		t.Errorf("stream returned the wrong aggregates\n\nwant: %#v\n\ngot: %#v\n\n", aggregates, as)
	}

	for _, a := range as {
		etest.AssertEqualEvents(
			t,
			event.Sort(getEvents(a.AggregateID()), event.SortAggregateVersion, event.SortAsc),
			getAppliedEvents(a.AggregateID()),
		)
	}
}

func TestStream_closedEventCursor(t *testing.T) {
	aggregates, getAppliedEvents := makeAggregates(3)
	aggregateMap := mapAggregates(aggregates)
	events, _ := makeEventsFor(aggregates, 10)

	ecur := ecursor.New(events...)
	ecur.Close(context.Background())

	cur := stream.New(
		context.Background(),
		ecur,
		stream.AggregateFactory("foo", func(id uuid.UUID) aggregate.Aggregate {
			return aggregateMap[id]
		}),
	)

	if ok := cur.Next(context.Background()); ok {
		t.Fatalf("cur.Next should return %t; got %t", false, ok)
	}

	if err := cur.Err(); !errors.Is(err, ecursor.ErrClosed) {
		t.Fatalf("cur.Err should return %#v; got %#v", ecursor.ErrClosed, err)
	}

	for _, a := range aggregates {
		if l := len(getAppliedEvents(a.AggregateID())); l != 0 {
			t.Errorf("no events should be applied; got %d", l)
		}
	}
}

func TestStream_Close(t *testing.T) {
	aggregates, getAppliedEvents := makeAggregates(3)
	aggregateMap := mapAggregates(aggregates)
	events, _ := makeEventsFor(aggregates, 10)

	ecur := xcursor.Delayed(ecursor.New(events...), 20*time.Millisecond)

	cur := stream.New(
		context.Background(),
		ecur,
		stream.AggregateFactory("foo", func(id uuid.UUID) aggregate.Aggregate {
			return aggregateMap[id]
		}),
	)

	closed := func() <-chan error {
		errc := make(chan error)
		go func() { errc <- cur.Close(context.Background()) }()
		return errc
	}()

	select {
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("cursor not closed after %s", 100*time.Millisecond)
	case err := <-closed:
		if err != nil {
			t.Fatalf("cur.Close should not fail: %v", err)
		}
	}

	select {
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("cur.Next didn't return after %s", 500*time.Millisecond)
	case ok := <-func() <-chan bool {
		done := make(chan bool)
		go func() {
			done <- cur.Next(context.Background())
		}()
		return done
	}():
		if ok {
			t.Fatalf("cur.Next should return %t; got %t", false, ok)
		}
	}

	if err := cur.Err(); !errors.Is(err, stream.ErrClosed) {
		t.Fatalf("cur.Err should return an %#v; got %#v", stream.ErrClosed, err)
	}

	for _, a := range aggregates {
		if l := len(getAppliedEvents(a.AggregateID())); l != 0 {
			t.Errorf("no events should be applied; got %d", l)
		}
	}
}

func TestStream_cancelContext(t *testing.T) {
	aggregates, getAppliedEvents := makeAggregates(3)
	aggregateMap := mapAggregates(aggregates)
	events, _ := makeEventsFor(aggregates, 10)

	ecur := xcursor.Delayed(ecursor.New(events...), 100*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())

	cur := stream.New(
		ctx,
		ecur,
		stream.AggregateFactory("foo", func(id uuid.UUID) aggregate.Aggregate {
			return aggregateMap[id]
		}),
	)

	cancel()
	<-time.After(50 * time.Millisecond)

	select {
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("cur.Next didn't return after %s", 500*time.Millisecond)
	case ok := <-func() <-chan bool {
		done := make(chan bool)
		go func() {
			done <- cur.Next(context.Background())
		}()
		return done
	}():
		if ok {
			t.Fatalf("cur.Next should return %t; got %t", false, ok)
		}
	}

	if err := cur.Err(); !errors.Is(err, stream.ErrClosed) {
		t.Fatalf("cur.Err should return an %#v; got %#v", stream.ErrClosed, err)
	}

	for _, a := range aggregates {
		if l := len(getAppliedEvents(a.AggregateID())); l != 0 {
			t.Errorf("no events should be applied; got %d", l)
		}
	}
}

func drainEvents(c <-chan event.Event) []event.Event {
	var events []event.Event
	for evt := range c {
		events = append(events, evt)
	}
	return events
}

func makeAggregates(n int) (_ []aggregate.Aggregate, getEvents func(uuid.UUID) []event.Event) {
	as := make([]aggregate.Aggregate, n)
	var mux sync.RWMutex
	applied := make(map[uuid.UUID][]event.Event)
	for i := range as {
		as[i] = test.NewFoo(uuid.New(), test.ApplyEventFunc("foo", func(evt event.Event) {
			mux.Lock()
			defer mux.Unlock()
			applied[evt.AggregateID()] = append(applied[evt.AggregateID()], evt)
		}))
	}

	return as, func(id uuid.UUID) []event.Event {
		mux.RLock()
		defer mux.RUnlock()
		return applied[id]
	}
}

func mapAggregates(as []aggregate.Aggregate) map[uuid.UUID]aggregate.Aggregate {
	m := make(map[uuid.UUID]aggregate.Aggregate)
	for _, a := range as {
		m[a.AggregateID()] = a
	}
	return m
}

type makeEventsConfig struct {
	skipVersions map[int]bool
}

func skipVersion(v ...int) func(*makeEventsConfig) {
	return func(cfg *makeEventsConfig) {
		for _, v := range v {
			cfg.skipVersions[v] = true
		}
	}
}

func makeEventsFor(as []aggregate.Aggregate, n int, opts ...func(*makeEventsConfig)) (_ []event.Event, getEvents func(uuid.UUID) []event.Event) {
	cfg := makeEventsConfig{skipVersions: make(map[int]bool)}
	for _, opt := range opts {
		opt(&cfg)
	}

	es := make([]event.Event, 0, n*len(as))
	var mux sync.RWMutex
	em := make(map[uuid.UUID][]event.Event, len(as))
	for _, a := range as {
		now := time.Now()
		events := make([]event.Event, n)
		var skipped int
		for j := range events {
			v := a.AggregateVersion() + 1 + j
			if cfg.skipVersions[v] {
				skipped++
			}
			v += skipped
			events[j] = event.New(
				"foo",
				etest.FooEventData{},
				event.Aggregate(a.AggregateName(), a.AggregateID(), v),
				event.Time(now.Add(time.Duration(j)*time.Hour)),
			)
		}
		es = append(es, events...)
		em[a.AggregateID()] = events
	}
	return es, func(id uuid.UUID) []event.Event {
		mux.RLock()
		defer mux.RUnlock()
		return em[id]
	}
}

func shuffleEvents(events []event.Event) []event.Event {
	shuffled := make([]event.Event, len(events))
	copy(shuffled, events)
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(shuffled), func(i, j int) {
		shuffled[j], shuffled[i] = shuffled[i], shuffled[j]
	})
	return shuffled
}
