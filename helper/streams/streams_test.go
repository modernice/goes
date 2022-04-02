package streams_test

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/test"
	"github.com/modernice/goes/helper/streams"
)

func TestBefore(t *testing.T) {
	original := []event.Event{
		event.New("foo", test.FooEventData{}).Any(),
		event.New("bar", test.BarEventData{}).Any(),
		event.New("baz", test.BazEventData{}).Any(),
	}
	add := []event.Event{
		event.New("foo", test.FooEventData{}).Any(),
		event.New("foobar", test.FoobarEventData{}).Any(),
	}

	str := streams.New(original)

	str = streams.Before(str, func(evt event.Event) []event.Event {
		if evt.Name() == "foo" || evt.Name() == "baz" {
			return add
		}
		return nil
	})

	events, err := streams.Drain(context.Background(), str)
	if err != nil {
		t.Fatalf("drain stream: %v", err)
	}

	want := append(append(add, original[:2]...), append(add, original[2])...)

	if !cmp.Equal(want, events) {
		t.Fatalf("stream returned wrong events\n%s", cmp.Diff(want, events))
	}
}
