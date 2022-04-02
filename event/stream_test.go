package event_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/query"
	"github.com/modernice/goes/event/test"
	"github.com/modernice/goes/helper/streams"
)

func TestStream(t *testing.T) {
	events := makeEvents()
	str := streams.New(events)

	result, err := streams.Drain(context.Background(), str)
	if err != nil {
		t.Fatal(fmt.Errorf("expected cur.Err to return %#v; got %#v", error(nil), err))
	}

	au := cmp.AllowUnexported(event.New[any]("foo", test.FooEventData{}))
	if !cmp.Equal(events, result, au) {
		t.Errorf(
			"expected cursor events to equal original events\noriginal: %#v\n\ngot: %#v\n\ndiff: %s",
			events,
			result,
			cmp.Diff(events[1:], result, au),
		)
	}
}

func TestDrain(t *testing.T) {
	events := makeEvents()
	str := streams.New(events)

	result, err := streams.Drain(context.Background(), str)
	if err != nil {
		t.Fatal(fmt.Errorf("expected cursor.Drain not to return an error; got %#v", err))
	}

	au := cmp.AllowUnexported(event.New[any]("foo", test.FooEventData{}))
	if !cmp.Equal(events, result, au) {
		t.Errorf(
			"expected cursor events to equal original events\noriginal: %#v\n\ngot: %#v\n\ndiff: %s",
			events,
			result,
			cmp.Diff(events, result, au),
		)
	}
}

func TestDrain_partial(t *testing.T) {
	events := makeEvents()
	str := streams.New(events)

	<-str

	result, err := streams.Drain(context.Background(), str)
	if err != nil {
		t.Fatal(fmt.Errorf("expected cursor.Drain not to return an error; got %v", err))
	}

	au := cmp.AllowUnexported(event.New[any]("foo", test.FooEventData{}))
	if !cmp.Equal(events[1:], result, au) {
		t.Errorf(
			"expected cursor events to equal original events\noriginal: %#v\n\ngot: %#v\n\ndiff: %s",
			events[1:],
			result,
			cmp.Diff(events[1:], result, au),
		)
	}
}

func TestWalk(t *testing.T) {
	events := makeEvents()
	str := streams.New(events)

	var walked []event.Event
	err := streams.Walk(context.Background(), func(evt event.Event) error {
		walked = append(walked, evt)
		return nil
	}, str)

	if err != nil {
		t.Fatalf("Walk shouldn't fail; failed with %q", err)
	}

	test.AssertEqualEvents(t, walked, events)
}

func TestWalk_chanError(t *testing.T) {
	events := makeEvents()
	errs := make(chan error, 1)
	mockError := errors.New("mock error")
	str := streams.New(events)

	errs <- mockError
	close(errs)

	err := streams.Walk(context.Background(), func(evt event.Event) error { return nil }, str, errs)

	if !errors.Is(err, mockError) {
		t.Errorf("Walk should fail with %q; got %q", mockError, err)
	}
}

func TestWalk_error(t *testing.T) {
	events := makeEvents()
	mockError := errors.New("mock error")
	str := streams.New(events)

	err := streams.Walk(context.Background(), func(evt event.Event) error { return mockError }, str)

	if !errors.Is(err, mockError) {
		t.Errorf("Walk should fail with %q; got %q", mockError, err)
	}
}

func TestFilter(t *testing.T) {
	events := []event.Event{
		event.New[any]("foo", test.FooEventData{}),
		event.New[any]("bar", test.FooEventData{}),
		event.New[any]("baz", test.FooEventData{}),
		event.New[any]("foobar", test.FooEventData{}),
		event.New[any]("barbaz", test.FooEventData{}),
		event.New[any]("foobaz", test.FooEventData{}),
	}

	str := streams.New(events)
	str = event.Filter(str, query.New(query.Name("bar", "baz", "barbaz", "foobaz")))
	str = event.Filter(str, query.New(query.Name("baz", "foobaz")))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	filtered, err := streams.Drain(ctx, str)
	if err != nil {
		t.Fatalf("drain Events: %v", err)
	}

	test.AssertEqualEvents(t, filtered, []event.Event{events[2], events[5]})
}

func makeEvents() []event.Event {
	return []event.Event{
		event.New[any]("foo", test.FooEventData{}),
		event.New[any]("bar", test.BarEventData{}),
		event.New[any]("baz", test.BazEventData{}),
	}
}
