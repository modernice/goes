package stream_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/stream"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/test"
	"github.com/modernice/goes/helper/pick"
	"github.com/modernice/goes/helper/streams"
	"github.com/modernice/goes/internal/xaggregate"
	"github.com/modernice/goes/internal/xevent"
	"github.com/modernice/goes/internal/xevent/xstream"
	"github.com/modernice/goes/internal/xtime"
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
	es := streams.New(events...)

	str, errs := stream.New(context.Background(), es)

	res, err := drain(str, errs, time.Second, makeFactory(am))
	if err != nil {
		t.Fatalf("drain stream: %v", err)
	}

	applied := getAppliedEvents(pick.AggregateID(as[0]))
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

	if v := pick.AggregateVersion(res[0]); v != 10 {
		t.Errorf("aggregate should have version %d; got %d", 10, v)
	}
}

func TestStream_singleAggregate_unsorted(t *testing.T) {
	as, getAppliedEvents := xaggregate.Make(1)
	am := xaggregate.Map(as)
	events := xevent.Make("foo", test.FooEventData{}, 10, xevent.ForAggregate(as...))
	events = xevent.Shuffle(events)
	es := streams.New(events...)

	str, errs := stream.New(context.Background(), es)

	res, err := drain(str, errs, time.Second, makeFactory(am))
	if err != nil {
		t.Fatalf("drain stream: %v", err)
	}

	applied := getAppliedEvents(pick.AggregateID(as[0]))
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
	es := streams.New(events...)

	str, errs := stream.New(context.Background(), es)

	res, err := drain(str, errs, 3*time.Second, makeFactory(am))
	if err != nil {
		t.Fatalf("drain stream: %v", err)
	}

	if len(res) != len(as) {
		t.Errorf("stream should return %d aggregates; got %d", len(as), len(res))
	}

	for _, a := range as {
		id, _, _ := a.Aggregate()
		applied := getAppliedEvents(id)
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

func TestStream_inconsistent(t *testing.T) {
	as, _ := xaggregate.Make(1)
	am := xaggregate.Map(as)
	events := xevent.Make("foo", test.FooEventData{}, 10, xevent.ForAggregate(as...), xevent.SkipVersion(3))

	es := streams.New(events...)
	str, errs := stream.New(context.Background(), es)

	res, err := drain(str, errs, 3*time.Second, makeFactory(am))

	if len(res) != 0 {
		t.Fatalf("stream should return no aggregates; got %d:\n\n%#v\n\n", len(res), res)
	}

	var cerr *aggregate.ConsistencyError
	if !errors.As(err, &cerr) {
		t.Fatalf("stream should return an error of type %T; got %T", cerr, err)
	}

	if pick.AggregateID(cerr.Aggregate) != pick.AggregateID(as[0]) {
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

	es := streams.New(events...)
	str, errs := stream.New(context.Background(), es, stream.Sorted(true))

	res, err := drain(str, errs, 3*time.Second, makeFactory(am))

	if len(res) != 0 {
		t.Errorf("stream should return no aggregates; got %d:\n\n%#v\n\n", len(res), res)
	}

	var cerr *aggregate.ConsistencyError
	if !errors.As(err, &cerr) {
		t.Errorf("stream should return an error of type %T; got %T", cerr, err)
	}

	if pick.AggregateID(cerr.Aggregate) != pick.AggregateID(as[0]) {
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

	es := streams.New(events...)
	es = xstream.Delayed(100*time.Millisecond, es)

	str, errs := stream.New(context.Background(), es, stream.Grouped(true))
	start := xtime.Now()
	select {
	case err, ok := <-errs:
		if !ok {
			t.Fatalf("Stream shouldn't be closed yet.")
		}
		t.Fatal(err)
	case _, ok := <-str:
		if !ok {
			t.Fatalf("Stream shouldn't be closed yet.")
		}
	}
	end := xtime.Now()
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

	es := streams.New(events...)
	str, errs := stream.New(context.Background(),
		es,
		// prevent sorting of events
		stream.Sorted(true),
		// disable consistency validation
		stream.ValidateConsistency(false),
	)

	res, err := drain(str, errs, 3*time.Second, makeFactory(am))

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

	es := streams.New(events...)

	str, errs := stream.New(context.Background(),
		es,
		stream.Filter(
			func(evt event.Event) bool {
				return strings.HasPrefix(pick.AggregateName(evt), "foo")
			},
			func(evt event.Event) bool {
				return strings.HasSuffix(pick.AggregateName(evt), "bar")
			},
		),
	)

	res, err := drain(str, errs, 3*time.Second, makeFactory(am))
	if err != nil {
		t.Errorf("stream should not return an error; got %v", err)
	}

	want := aggregate.Sort(foobars, aggregate.SortID, aggregate.SortAsc)
	got := aggregate.Sort(res, aggregate.SortID, aggregate.SortAsc)

	if !reflect.DeepEqual(want, got) {
		t.Errorf("stream returned the wrong aggregates\n\nwant: %#v\n\ngot: %#v\n\n", want, got)
	}
}

func drain(
	s <-chan aggregate.History,
	errs <-chan error,
	timeout time.Duration,
	factory func(string, uuid.UUID) aggregate.Aggregate,
) ([]aggregate.Aggregate, error) {
	var (
		ctx    context.Context
		cancel context.CancelFunc
	)
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), timeout)
	} else {
		ctx, cancel = context.WithCancel(context.Background())
	}
	defer cancel()

	results, err := streams.Drain(ctx, s, errs)
	if err != nil {
		return nil, err
	}

	as := make([]aggregate.Aggregate, len(results))
	for i, res := range results {
		as[i] = factory(res.AggregateName(), res.AggregateID())
		res.Apply(as[i])
	}

	return as, nil
}

func makeFactory(am map[uuid.UUID]aggregate.Aggregate) func(string, uuid.UUID) aggregate.Aggregate {
	return func(_ string, id uuid.UUID) aggregate.Aggregate {
		return am[id]
	}
}
