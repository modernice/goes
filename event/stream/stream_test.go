package stream_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/stream"
	"github.com/modernice/goes/event/test"
)

func TestInMemory(t *testing.T) {
	events := makeEvents()
	cur := stream.InMemory(events...)

	var cursorEvents []event.Event
	for cur.Next(context.Background()) {
		cursorEvents = append(cursorEvents, cur.Event())
	}

	if err := cur.Err(); err != nil {
		t.Fatal(fmt.Errorf("expected cur.Err to return %#v; got %#v", error(nil), err))
	}

	au := cmp.AllowUnexported(event.New("foo", test.FooEventData{}))
	if !cmp.Equal(events, cursorEvents, au) {
		t.Errorf(
			"expected cursor events to equal original events\noriginal: %#v\n\ngot: %#v\n\ndiff: %s",
			events,
			cursorEvents,
			cmp.Diff(events[1:], cursorEvents, au),
		)
	}

	if err := cur.Close(context.Background()); err != nil {
		t.Fatal(fmt.Errorf("expected cur.Close not to return an error; got %v", err))
	}
}

func TestStream_Next_closed(t *testing.T) {
	cur := stream.InMemory(makeEvents()...)
	if err := cur.Close(context.Background()); err != nil {
		t.Fatal(fmt.Errorf("expected cur.Close not to return an error; got %v", err))
	}

	if ok := cur.Next(context.Background()); ok {
		t.Errorf("expected cur.Next to return %t; got %t", false, ok)
	}

	if err := cur.Err(); !errors.Is(err, stream.ErrClosed) {
		t.Error(fmt.Errorf("expected cur.Err to return %#v; got %#v", stream.ErrClosed, err))
	}

	if evt := cur.Event(); evt != nil {
		t.Error(fmt.Errorf("expected cur.Event to return %#v; got %#v", event.Event(nil), evt))
	}
}

func TestAll(t *testing.T) {
	events := makeEvents()
	cur := stream.InMemory(events...)

	all, err := stream.All(context.Background(), cur)
	if err != nil {
		t.Fatal(fmt.Errorf("expected cursor.All not to return an error; got %#v", err))
	}

	au := cmp.AllowUnexported(event.New("foo", test.FooEventData{}))
	if !cmp.Equal(events, all, au) {
		t.Errorf(
			"expected cursor events to equal original events\noriginal: %#v\n\ngot: %#v\n\ndiff: %s",
			events,
			all,
			cmp.Diff(events, all, au),
		)
	}

	if ok := cur.Next(context.Background()); ok {
		t.Errorf("expected cur.Next to return %t; got %t", false, ok)
	}

	if err = cur.Err(); !errors.Is(err, stream.ErrClosed) {
		t.Errorf("expected cur.Err to return %#v; got %#v", stream.ErrClosed, err)
	}
}

func TestAll_partial(t *testing.T) {
	events := makeEvents()
	cur := stream.InMemory(events...)
	if !cur.Next(context.Background()) {
		t.Fatal(fmt.Errorf("cur.Next: %w", cur.Err()))
	}

	all, err := stream.All(context.Background(), cur)
	if err != nil {
		t.Fatal(fmt.Errorf("expected cursor.All not to return an error; got %v", err))
	}

	au := cmp.AllowUnexported(event.New("foo", test.FooEventData{}))
	if !cmp.Equal(events[1:], all, au) {
		t.Errorf(
			"expected cursor events to equal original events\noriginal: %#v\n\ngot: %#v\n\ndiff: %s",
			events[1:],
			all,
			cmp.Diff(events[1:], all, au),
		)
	}
}

func TestAll_closed(t *testing.T) {
	events := makeEvents()
	cur := stream.InMemory(events...)

	if err := cur.Close(context.Background()); err != nil {
		t.Fatal(fmt.Errorf("expected cur.Close not to return an error; got %#v", err))
	}

	if _, err := stream.All(context.Background(), cur); !errors.Is(err, stream.ErrClosed) {
		t.Errorf("expected cursor.All to return %#v; got %#v", stream.ErrClosed, err)
	}
}

func makeEvents() []event.Event {
	return []event.Event{
		event.New("foo", test.FooEventData{}),
		event.New("bar", test.BarEventData{}),
		event.New("baz", test.BazEventData{}),
	}
}
