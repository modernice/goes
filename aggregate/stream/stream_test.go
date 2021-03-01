package stream_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/consistency"
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

	str := stream.New(es)

	res, err := drain(str, time.Second, makeFactory(am))
	if err != nil {
		t.Fatalf("drain stream: %v", err)
	}

	applied := getAppliedEvents(as[0].AggregateID())
	test.AssertEqualEvents(t, xevent.FilterAggregate(events, as[0]), applied)

	if len(res) != 1 {
		t.Fatalf("stream should return 1 aggregate; got %d", len(res))
	}

	if res[0] != as[0] {
		t.Errorf("stream returned the wrong aggregate\n\nwant: %#v\n\ngot: %#v\n\n", as[0], res[0])
	}

	if l := len(res[0].AggregateChanges()); l != 0 {
		t.Errorf("stream should flush aggregate changes. len(changes)=%d", l)
	}

	if v := res[0].AggregateVersion(); v != 10 {
		t.Errorf("aggregate should have version %d; got %d", 10, v)
	}
}

func TestStream_singleAggregate_unsorted(t *testing.T) {
	as, getAppliedEvents := xaggregate.Make(1)
	am := xaggregate.Map(as)
	events := xevent.Make("foo", test.FooEventData{}, 10, xevent.ForAggregate(as...))
	events = xevent.Shuffle(events)
	es := estream.InMemory(events...)

	str := stream.New(es)

	res, err := drain(str, time.Second, makeFactory(am))
	if err != nil {
		t.Fatalf("drain stream: %v", err)
	}

	applied := getAppliedEvents(as[0].AggregateID())
	test.AssertEqualEvents(t, event.Sort(events, event.SortAggregateVersion, event.SortAsc), applied)

	if len(res) != 1 {
		t.Fatalf("stream should return 1 aggregate; got %d", len(res))
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

	str := stream.New(
		es,
	)

	res, err := drain(str, 3*time.Second, makeFactory(am))
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
	// am := xaggregate.Map(as)
	events := xevent.Make("foo", test.FooEventData{}, 10, xevent.ForAggregate(as...))
	events = xevent.Shuffle(events)

	es := xstream.Delayed(estream.InMemory(events...), time.Second)
	str := stream.New(es)

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
	events := xevent.Make("foo", test.FooEventData{}, 10, xevent.ForAggregate(as...))

	// delay the event cursor by 1 second, so that the no event can be pulled from
	// the stream
	es := xstream.Delayed(estream.InMemory(events...), time.Second)

	str := stream.New(
		es,
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

	name, id := str.Current()
	if name != "" || id != uuid.Nil {
		t.Errorf("stream should not return an aggregate; got [%s, %s]", name, id)
	}
}

func TestStream_Close_closedEventStream(t *testing.T) {
	as, _ := xaggregate.Make(10)
	// am := xaggregate.Map(as)
	events := xevent.Make("foo", test.FooEventData{}, 10, xevent.ForAggregate(as...))

	// delay the event cursor by 1 second, so that the no event can be pulled from
	// the stream
	es := xstream.Delayed(estream.InMemory(events...), time.Second)
	str := stream.New(es)

	if err := es.Close(context.Background()); err != nil {
		t.Fatalf("event stream failed to close: %v", err)
	}

	if ok := str.Next(context.Background()); ok {
		t.Errorf("str.Next should return %t; got %t", false, ok)
	}

	if err := str.Err(); !errors.Is(err, estream.ErrClosed) {
		t.Errorf("stream should return error %#v; got %#v", estream.ErrClosed, err)
	}

	name, id := str.Current()
	if name != "" || id != uuid.Nil {
		t.Errorf("stream should not return an aggregate; got [%s, %s]", name, id)
	}
}

func TestStream_Close_closeEventStream(t *testing.T) {
	as, _ := xaggregate.Make(10)
	events := xevent.Make("foo", test.FooEventData{}, 10, xevent.ForAggregate(as...))

	es := estream.InMemory(events...)
	str := stream.New(es)

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

	mes := mock_event.NewMockStream(ctrl)
	mockError := errors.New("mock error")
	mes.EXPECT().Close(gomock.Any()).Return(mockError)
	es := xstream.Delayed(mes, time.Second)

	str := stream.New(es)

	if err := str.Close(context.Background()); !errors.Is(err, mockError) {
		t.Errorf("str.Close should return the event stores error %#v; got %#v", mockError, err)
	}
}

func TestStream_Close_closeEventStream_contextCanceled(t *testing.T) {
	as, _ := xaggregate.Make(10)
	// am := xaggregate.Map(as)
	events := xevent.Make("foo", test.FooEventData{}, 10, xevent.ForAggregate(as...))

	es := estream.InMemory(events...)
	str := stream.New(es)

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
	str := stream.New(es)

	res, err := drain(str, 3*time.Second, makeFactory(am))

	if len(res) != 0 {
		t.Fatalf("stream should return no aggregates; got %d:\n\n%#v\n\n", len(res), res)
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

func TestSorted(t *testing.T) {
	as, _ := xaggregate.Make(1)
	am := xaggregate.Map(as)
	events := xevent.Make("foo", test.FooEventData{}, 10, xevent.ForAggregate(as...))
	events = event.Sort(events, event.SortAggregateVersion, event.SortAsc)

	// swap first and last event, so they're unordered
	events[0], events[len(events)-1] = events[len(events)-1], events[0]

	es := estream.InMemory(events...)
	str := stream.New(
		es,
		stream.Sorted(true),
	)

	res, err := drain(str, 3*time.Second, makeFactory(am))

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

func TestGrouped(t *testing.T) {
	as, _ := xaggregate.Make(3)
	as = aggregate.SortMulti(
		as,
		aggregate.SortOptions{Sort: aggregate.SortName, Dir: aggregate.SortAsc},
		aggregate.SortOptions{Sort: aggregate.SortID, Dir: aggregate.SortAsc},
	)
	events := xevent.Make("foo", test.FooEventData{}, 3, xevent.ForAggregate(as...))
	events = event.SortMulti(
		events,
		event.SortOptions{Sort: event.SortAggregateName, Dir: event.SortAsc},
		event.SortOptions{Sort: event.SortAggregateID, Dir: event.SortAsc},
		event.SortOptions{Sort: event.SortAggregateVersion, Dir: event.SortAsc},
	)

	es := estream.InMemory(events...)
	es = xstream.Delayed(es, 100*time.Millisecond)

	str := stream.New(es, stream.Grouped(true))
	start := time.Now()
	if !str.Next(context.Background()) {
		t.Fatalf("Next should return %t; got %t", true, false)
	}
	end := time.Now()
	dur := end.Sub(start)

	if dur < 300*time.Millisecond || dur > 600*time.Millisecond {
		t.Errorf(
			"Stream should return first Aggregate between ~[%s, %s]; was %s",
			300*time.Millisecond,
			600*time.Millisecond,
			dur,
		)
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
	str := stream.New(
		es,
		// prevent sorting of events
		stream.Sorted(true),
		// disable consistency validation
		stream.ValidateConsistency(false),
	)

	res, err := drain(str, 3*time.Second, makeFactory(am))

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

func TestFilter(t *testing.T) {
	foos, _ := xaggregate.Make(5, xaggregate.Name("foo"))
	foobars, _ := xaggregate.Make(5, xaggregate.Name("foobar"))
	as := append(foos, foobars...)
	am := xaggregate.Map(as)

	events := xevent.Make("foo", test.FooEventData{}, 10, xevent.ForAggregate(as...))
	events = xevent.Shuffle(events)

	es := estream.InMemory(events...)

	str := stream.New(
		es,
		stream.Filter(
			func(evt event.Event) bool {
				return strings.HasPrefix(evt.AggregateName(), "foo")
			},
			func(evt event.Event) bool {
				return strings.HasSuffix(evt.AggregateName(), "bar")
			},
		),
	)

	res, err := drain(str, 3*time.Second, makeFactory(am))
	if err != nil {
		t.Errorf("stream should not return an error; got %v", err)
	}

	want := aggregate.Sort(foobars, aggregate.SortID, aggregate.SortAsc)
	got := aggregate.Sort(res, aggregate.SortID, aggregate.SortAsc)

	if !reflect.DeepEqual(want, got) {
		t.Errorf("stream returned the wrong aggregates\n\nwant: %#v\n\ngot: %#v\n\n", want, got)
	}
}

func drain(s aggregate.Stream, timeout time.Duration, factory func(string, uuid.UUID) aggregate.Aggregate) ([]aggregate.Aggregate, error) {
	type result struct {
		as  []aggregate.Aggregate
		err error
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	res := make(chan result)

	go func() {
		defer s.Close(ctx)
		var as []aggregate.Aggregate
		var err error
		for s.Next(ctx) {
			name, id := s.Current()
			a := factory(name, id)
			if err := s.Apply(a); err != nil {
				res <- result{
					as:  as,
					err: err,
				}
				return
			}
			as = append(as, a)
		}
		if s.Err() != nil {
			err = fmt.Errorf("stream: %w", err)
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

func makeFactory(am map[uuid.UUID]aggregate.Aggregate) func(string, uuid.UUID) aggregate.Aggregate {
	return func(_ string, id uuid.UUID) aggregate.Aggregate {
		return am[id]
	}
}
