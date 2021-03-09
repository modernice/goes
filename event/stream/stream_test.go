package stream_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/stream"
	"github.com/modernice/goes/event/test"
)

func TestSlice(t *testing.T) {
	events := makeEvents()
	str := stream.Slice(events...)

	result, err := stream.Drain(context.Background(), str)
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
	str := stream.Slice(events...)

	result, err := stream.Drain(context.Background(), str)
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
	str := stream.Slice(events...)

	<-str

	result, err := stream.Drain(context.Background(), str)
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

func makeEvents() []event.Event {
	return []event.Event{
		event.New("foo", test.FooEventData{}),
		event.New("bar", test.BarEventData{}),
		event.New("baz", test.BazEventData{}),
	}
}
