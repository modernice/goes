package stream_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/consistency"
	"github.com/modernice/goes/aggregate/factory"
	"github.com/modernice/goes/aggregate/stream"
	"github.com/modernice/goes/event"
	mock_event "github.com/modernice/goes/event/mocks"
	estream "github.com/modernice/goes/event/stream"
	"github.com/modernice/goes/event/test"
	"github.com/modernice/goes/internal/xaggregate"
	"github.com/modernice/goes/internal/xevent"
	"github.com/modernice/goes/internal/xevent/xstream"
)

type makeEventsOption func(*makeEventsConfig)

type makeEventsConfig struct {
	skip []int
}

func TestStream_singleAggregate_sorted(t *testing.T) {
	as, getAppliedEvents := xaggregate.Make(1)
	am := xaggregate.Map(as)
	events := xevent.Make("foo", test.FooEventData{}, 10, xevent.ForAggregate(as...))
	events = event.Sort(events, event.SortAggregateVersion, event.SortAsc)
	es := estream.InMemory(events...)

	str := stream.FromEvents(
		es,
		stream.Factory(newFactory("foo", func(id uuid.UUID) aggregate.Aggregate {
			return am[id]
		})),
	)

	res, err := drain(str, time.Second)
	if err != nil {
		t.Fatalf("drain stream: %v", err)
	}

	applied := getAppliedEvents(as[0].AggregateID())
	test.AssertEqualEvents(t, xevent.FilterAggregate(events, as[0]), applied)

	if len(res) != 1 {
		t.Errorf("stream should return 1 aggregate; got %d", len(res))
	}

	if res[0] != as[0] {
		t.Errorf("stream returned the wrong aggregate\n\nwant: %#v\n\ngot: %#v\n\n", as[0], res[0])
	}
}

func TestStream_singleAggregate_unsorted(t *testing.T) {
	as, getAppliedEvents := xaggregate.Make(1)
	am := xaggregate.Map(as)
	events := xevent.Make("foo", test.FooEventData{}, 10, xevent.ForAggregate(as...))
	events = xevent.Shuffle(events)
	es := estream.InMemory(events...)

	str := stream.FromEvents(
		es,
		stream.Factory(newFactory("foo", func(id uuid.UUID) aggregate.Aggregate {
			return am[id]
		})),
	)

	res, err := drain(str, time.Second)
	if err != nil {
		t.Fatalf("drain stream: %v", err)
	}

	applied := getAppliedEvents(as[0].AggregateID())
	test.AssertEqualEvents(t, event.Sort(events, event.SortAggregateVersion, event.SortAsc), applied)

	if len(res) != 1 {
		t.Errorf("stream should return 1 aggregate; got %d", len(res))
	}

	if res[0] != as[0] {
		t.Errorf("stream returned the wrong aggregate\n\nwant: %#v\n\ngot: %#v\n\n", as[0], res[0])
	}
}

func TestStream_multipleAggregates_unsorted(t *testing.T) {
	as, getAppliedEvents := xaggregate.Make(10)
	am := xaggregate.Map(as)
	events := xevent.Make("foo", test.FooEventData{}, 10, xevent.ForAggregate(as...))
	events = xevent.Shuffle(events)
	es := estream.InMemory(events...)

	str := stream.FromEvents(
		es,
		stream.Factory(newFactory("foo", func(id uuid.UUID) aggregate.Aggregate {
			return am[id]
		})),
	)

	res, err := drain(str, 3*time.Second)
	if err != nil {
		t.Fatalf("drain stream: %v", err)
	}

	if len(res) != len(as) {
		t.Errorf("stream should return %d aggregates; got %d", len(as), len(res))
	}

	for _, a := range as {
		applied := getAppliedEvents(a.AggregateID())
		test.AssertEqualEvents(t, event.Sort(
			xevent.FilterAggregate(events, a),
			event.SortAggregateVersion,
			event.SortAsc,
		), applied)

		var found bool
		for _, ra := range res {
			if ra == a {
				found = true
				break
			}
		}

		if !found {
			t.Errorf("stream did not return %#v", a)
		}
	}
}

func TestStream_Next_contextCanceled(t *testing.T) {
	as, _ := xaggregate.Make(3)
	am := xaggregate.Map(as)
	events := xevent.Make("foo", test.FooEventData{}, 10, xevent.ForAggregate(as...))
	events = xevent.Shuffle(events)

	es := xstream.Delayed(estream.InMemory(events...), time.Second)

	str := stream.FromEvents(
		es,
		stream.Factory(newFactory("foo", func(id uuid.UUID) aggregate.Aggregate {
			return am[id]
		})),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	if ok := str.Next(ctx); ok {
		t.Errorf("str.Next should return %t; got %t", false, ok)
	}

	if err := str.Err(); err == nil || !errors.Is(err, ctx.Err()) {
		t.Errorf("stream should return error %#v; got %#v", ctx.Err(), err)
	}
}

func TestStream_Close(t *testing.T) {
	as, _ := xaggregate.Make(10)
	am := xaggregate.Map(as)
	events := xevent.Make("foo", test.FooEventData{}, 10, xevent.ForAggregate(as...))

	// delay the event cursor by 1 second, so that the no event can be pulled from
	// the stream
	es := xstream.Delayed(estream.InMemory(events...), time.Second)

	str := stream.FromEvents(
		es,
		stream.Factory(newFactory("foo", func(id uuid.UUID) aggregate.Aggregate {
			return am[id]
		})),
	)

	if err := str.Close(context.Background()); err != nil {
		t.Fatalf("stream failed to close: %v", err)
	}

	if ok := str.Next(context.Background()); ok {
		t.Errorf("str.Next should return %t; got %t", false, ok)
	}

	if err := str.Err(); !errors.Is(err, stream.ErrClosed) {
		t.Errorf("stream should return error %#v; got %#v", stream.ErrClosed, err)
	}

	if a := str.Aggregate(); a != nil {
		t.Errorf("stream should not return an aggregate; got %#v", a)
	}
}

func TestStream_Close_closedEventStream(t *testing.T) {
	as, _ := xaggregate.Make(10)
	am := xaggregate.Map(as)
	events := xevent.Make("foo", test.FooEventData{}, 10, xevent.ForAggregate(as...))

	// delay the event cursor by 1 second, so that the no event can be pulled from
	// the stream
	es := xstream.Delayed(estream.InMemory(events...), time.Second)

	str := stream.FromEvents(
		es,
		stream.Factory(newFactory("foo", func(id uuid.UUID) aggregate.Aggregate {
			return am[id]
		})),
	)

	if err := es.Close(context.Background()); err != nil {
		t.Fatalf("event stream failed to close: %v", err)
	}

	if ok := str.Next(context.Background()); ok {
		t.Errorf("str.Next should return %t; got %t", false, ok)
	}

	if err := str.Err(); !errors.Is(err, estream.ErrClosed) {
		t.Errorf("stream should return error %#v; got %#v", estream.ErrClosed, err)
	}

	if a := str.Aggregate(); a != nil {
		t.Errorf("stream should not return an aggregate; got %#v", a)
	}
}

func TestStream_Close_closeEventStream(t *testing.T) {
	as, _ := xaggregate.Make(10)
	am := xaggregate.Map(as)
	events := xevent.Make("foo", test.FooEventData{}, 10, xevent.ForAggregate(as...))

	es := estream.InMemory(events...)
	str := stream.FromEvents(es, stream.Factory(newFactory("foo", func(id uuid.UUID) aggregate.Aggregate {
		return am[id]
	})))

	if err := str.Close(context.Background()); err != nil {
		t.Errorf("stream failed to close: %v", err)
	}

	if es.Next(context.Background()) {
		t.Errorf("es.Next should return %t; got %t", false, true)
	}

	if err := es.Err(); !errors.Is(err, estream.ErrClosed) {
		t.Errorf("event stream should be closed and return error %#v; got %#v", estream.ErrClosed, err)
	}
}

func TestStream_Close_closeEventStream_error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	as, _ := xaggregate.Make(10)
	am := xaggregate.Map(as)

	mes := mock_event.NewMockStream(ctrl)
	mockError := errors.New("mock error")
	mes.EXPECT().Close(gomock.Any()).Return(mockError)
	es := xstream.Delayed(mes, time.Second)

	str := stream.FromEvents(es, stream.Factory(newFactory("foo", func(id uuid.UUID) aggregate.Aggregate {
		return am[id]
	})))

	if err := str.Close(context.Background()); !errors.Is(err, mockError) {
		t.Errorf("str.Close should return the event stores error %#v; got %#v", mockError, err)
	}
}

func TestStream_Close_closeEventStream_contextCanceled(t *testing.T) {
	as, _ := xaggregate.Make(10)
	am := xaggregate.Map(as)
	events := xevent.Make("foo", test.FooEventData{}, 10, xevent.ForAggregate(as...))

	es := estream.InMemory(events...)
	str := stream.FromEvents(es, stream.Factory(newFactory("foo", func(id uuid.UUID) aggregate.Aggregate {
		return am[id]
	})))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := str.Close(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("str.Close should return error %v; got %v", context.Canceled, err)
	}
}

func TestStream_inconsistent(t *testing.T) {
	as, _ := xaggregate.Make(1)
	am := xaggregate.Map(as)
	events := xevent.Make("foo", test.FooEventData{}, 10, xevent.ForAggregate(as...), xevent.SkipVersion(3))

	es := estream.InMemory(events...)

	str := stream.FromEvents(
		es,
		stream.Factory(newFactory("foo", func(id uuid.UUID) aggregate.Aggregate {
			return am[id]
		})),
	)

	res, err := stream.All(context.Background(), str)

	if len(res) != 0 {
		t.Errorf("stream should return no aggregates; got %d:\n\n%#v\n\n", len(res), res)
	}

	var cerr *consistency.Error
	if !errors.As(err, &cerr) {
		t.Errorf("stream should return an error of type %T; got %T", cerr, err)
	}

	if cerr.Aggregate != as[0] {
		t.Errorf("cerr.Aggregate should be %#v; got %#v", as[0], cerr.Aggregate)
	}

	if cerr.Event() != events[2] {
		t.Errorf("cerr.Event should return %#v; got %#v", events[2], cerr.Event())
	}
}

func TestIsSorted(t *testing.T) {
	as, _ := xaggregate.Make(1)
	am := xaggregate.Map(as)
	events := xevent.Make("foo", test.FooEventData{}, 10, xevent.ForAggregate(as...))
	events = event.Sort(events, event.SortAggregateVersion, event.SortAsc)

	// swap first and last event, so they're unordered
	events[0], events[len(events)-1] = events[len(events)-1], events[0]

	es := estream.InMemory(events...)
	str := stream.FromEvents(
		es,
		stream.Factory(newFactory("foo", func(id uuid.UUID) aggregate.Aggregate {
			return am[id]
		})),
		stream.IsSorted(true),
	)

	res, err := stream.All(context.Background(), str)

	if len(res) != 0 {
		t.Errorf("stream should return no aggregates; got %d:\n\n%#v\n\n", len(res), res)
	}

	var cerr *consistency.Error
	if !errors.As(err, &cerr) {
		t.Errorf("stream should return an error of type %T; got %T", cerr, err)
	}

	if cerr.Aggregate != as[0] {
		t.Errorf("cerr.Aggregate should be %#v; got %#v", as[0], cerr.Aggregate)
	}

	if cerr.Event() != events[0] {
		t.Errorf("cerr.Event should return %#v; got %#v", events[0], cerr.Event())
	}
}

func TestIsGrouped(t *testing.T) {
	as, _ := xaggregate.Make(2)
	as = aggregate.SortMulti(
		as,
		aggregate.SortOptions{Sort: aggregate.SortName, Dir: aggregate.SortAsc},
		aggregate.SortOptions{Sort: aggregate.SortID, Dir: aggregate.SortAsc},
	)

	am := xaggregate.Map(as)
	events := xevent.Make("foo", test.FooEventData{}, 5, xevent.ForAggregate(as...))
	events = event.SortMulti(
		events,
		event.SortOptions{Sort: event.SortAggregateName, Dir: event.SortAsc},
		event.SortOptions{Sort: event.SortAggregateID, Dir: event.SortAsc},
		event.SortOptions{Sort: event.SortAggregateVersion, Dir: event.SortAsc},
	)

	// Delay the event stream by 10ms until all events for the first aggregate
	// have been received. Then delay the remaining events by 1s, so that they
	// never arrive.
	var handledFirstAggregate bool
	es := xstream.DelayedFunc(estream.InMemory(events...), func(prev event.Event) time.Duration {
		if prev == events[4] {
			handledFirstAggregate = true
			return 10 * time.Millisecond
		}
		if handledFirstAggregate {
			return time.Second
		}
		return 10 * time.Millisecond
	})

	str := stream.FromEvents(
		es,
		stream.Factory(newFactory("foo", func(id uuid.UUID) aggregate.Aggregate {
			return am[id]
		})),
		stream.IsGrouped(true),
	)

	// Add a 500ms timeout to ensure the second aggregate isn't built.
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	if !str.Next(ctx) {
		t.Fatalf("str.Next should return %t; got %t", true, false)
	}

	if err := str.Err(); err != nil {
		t.Errorf("stream should return no error; got %#v", err)
	}

	a := str.Aggregate()
	if a != as[0] {
		t.Errorf("str.Aggregate should return %#v; got %#v", as[0], a)
	}

	if str.Next(ctx) {
		t.Errorf("str.Next should return %t; got %t", false, true)
	}

	err := str.Err()
	if err == nil {
		t.Errorf("str.Err should return an error; got <nil>")
	}

	if !errors.Is(err, ctx.Err()) {
		t.Errorf("str.Err should return %#v; got %#v", ctx.Err(), err)
	}
}

func TestValidateConsistency(t *testing.T) {
	as, _ := xaggregate.Make(1)
	am := xaggregate.Map(as)
	events := xevent.Make("foo", test.FooEventData{}, 10, xevent.ForAggregate(as...))
	events = event.Sort(events, event.SortAggregateVersion, event.SortAsc)

	// swap first and last event, so they're unordered
	events[0], events[len(events)-1] = events[len(events)-1], events[0]

	es := estream.InMemory(events...)
	str := stream.FromEvents(
		es,
		stream.Factory(newFactory("foo", func(id uuid.UUID) aggregate.Aggregate {
			return am[id]
		})),

		// prevent sorting of events
		stream.IsSorted(true),
		// disable consistency validation
		stream.ValidateConsistency(false),
	)

	res, err := stream.All(context.Background(), str)
	if err != nil {
		t.Errorf("stream should return no error; got %#v", err)
	}

	if len(res) != 1 {
		t.Errorf("stream should return 1 aggregate; got %d:\n\n%#v\n\n", len(res), res)
	}

	if res[0] != as[0] {
		t.Errorf("stream returned the wrong aggregate:\n\nwant: %#v\n\ngot %#v\n\n", as[0], res[0])
	}
}

func TestFactory(t *testing.T) {
	as, _ := xaggregate.Make(1)
	events := xevent.Make("foo", test.FooEventData{}, 10, xevent.ForAggregate(as...))
	events = event.Sort(events, event.SortAggregateVersion, event.SortAsc)

	es := estream.InMemory(events...)
	f := factory.New()
	str := stream.FromEvents(es, stream.Factory(f))

	if str.Next(context.Background()) {
		t.Errorf("str.Next should return %t; got %t", false, true)
	}

	if err := str.Err(); !errors.Is(err, factory.ErrUnknownName) {
		t.Errorf("str.Err should return %#v; got %#v", factory.ErrUnknownName, err)
	}
}

func drain(s aggregate.Stream, timeout time.Duration) ([]aggregate.Aggregate, error) {
	type result struct {
		as  []aggregate.Aggregate
		err error
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	res := make(chan result)

	go func() {
		as, err := stream.All(ctx, s)
		if err != nil {
			err = fmt.Errorf("stream returned an error: %w", err)
		}
		res <- result{
			as:  as,
			err: err,
		}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case r := <-res:
		return r.as, r.err
	}
}

func newFactory(name string, fn func(uuid.UUID) aggregate.Aggregate) aggregate.Factory {
	return factory.New(factory.For(name, fn))
}
