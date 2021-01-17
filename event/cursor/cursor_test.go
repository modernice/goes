package cursor_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/cursor"
	"github.com/modernice/goes/event/test"
)

func TestCursor(t *testing.T) {
	events := makeEvents()
	cur := cursor.New(events...)

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

func TestCursor_Next_closed(t *testing.T) {
	cur := cursor.New(makeEvents()...)
	if err := cur.Close(context.Background()); err != nil {
		t.Fatal(fmt.Errorf("expected cur.Close not to return an error; got %v", err))
	}

	if ok := cur.Next(context.Background()); ok {
		t.Errorf("expected cur.Next to return %t; got %t", false, ok)
	}

	if err := cur.Err(); !errors.Is(err, cursor.ErrClosed) {
		t.Error(fmt.Errorf("expected cur.Err to return %#v; got %#v", cursor.ErrClosed, err))
	}

	if evt := cur.Event(); evt != nil {
		t.Error(fmt.Errorf("expected cur.Event to return %#v; got %#v", event.Event(nil), evt))
	}
}

func TestAll(t *testing.T) {
	events := makeEvents()
	cur := cursor.New(events...)

	all, err := cursor.All(context.Background(), cur)
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

	if err = cur.Err(); !errors.Is(err, cursor.ErrClosed) {
		t.Errorf("expected cur.Err to return %#v; got %#v", cursor.ErrClosed, err)
	}
}

func TestAll_partial(t *testing.T) {
	events := makeEvents()
	cur := cursor.New(events...)
	if !cur.Next(context.Background()) {
		t.Fatal(fmt.Errorf("cur.Next: %w", cur.Err()))
	}

	all, err := cursor.All(context.Background(), cur)
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
	cur := cursor.New(events...)

	if err := cur.Close(context.Background()); err != nil {
		t.Fatal(fmt.Errorf("expected cur.Close not to return an error; got %#v", err))
	}

	if _, err := cursor.All(context.Background(), cur); !errors.Is(err, cursor.ErrClosed) {
		t.Errorf("expected cursor.All to return %#v; got %#v", cursor.ErrClosed, err)
	}
}

func makeEvents() []event.Event {
	return []event.Event{
		event.New("foo", test.FooEventData{}),
		event.New("bar", test.BarEventData{}),
		event.New("baz", test.BazEventData{}),
	}
}
