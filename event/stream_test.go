package event_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/test"
)

func TestStream(t *testing.T) {
	events := makeEvents()
	str := event.Stream(events...)

	result, err := event.Drain(context.Background(), str)
	if err != nil {
		t.Fatal(fmt.Errorf("expected cur.Err to return %#v; got %#v", error(nil), err))
	}

	au := cmp.AllowUnexported(event.New("foo", test.FooEventData{}))
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
	str := event.Stream(events...)

	result, err := event.Drain(context.Background(), str)
	if err != nil {
		t.Fatal(fmt.Errorf("expected cursor.Drain not to return an error; got %#v", err))
	}

	au := cmp.AllowUnexported(event.New("foo", test.FooEventData{}))
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
	str := event.Stream(events...)

	<-str

	result, err := event.Drain(context.Background(), str)
	if err != nil {
		t.Fatal(fmt.Errorf("expected cursor.Drain not to return an error; got %v", err))
	}

	au := cmp.AllowUnexported(event.New("foo", test.FooEventData{}))
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
	str := event.Stream(events...)

	var walked []event.Event
	err := event.Walk(context.Background(), func(evt event.Event) {
		walked = append(walked, evt)
	}, str)

	if err != nil {
		t.Fatalf("Walk shouldn't fail; failed with %q", err)
	}

	test.AssertEqualEvents(t, walked, events)
}

func TestWalk_error(t *testing.T) {
	events := makeEvents()
	errs := make(chan error, 1)
	mockError := errors.New("mock error")
	str := event.Stream(events...)

	errs <- mockError
	close(errs)

	err := event.Walk(context.Background(), func(evt event.Event) {}, str, errs)

	if !errors.Is(err, mockError) {
		t.Errorf("Walk should fail with %q; got %q", mockError, err)
	}
}

func makeEvents() []event.Event {
	return []event.Event{
		event.New("foo", test.FooEventData{}),
		event.New("bar", test.BarEventData{}),
		event.New("baz", test.BazEventData{}),
	}
}
