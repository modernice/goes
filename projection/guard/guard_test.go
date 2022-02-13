package guard_test

import (
	"testing"

	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/test"
	"github.com/modernice/goes/projection/guard"
)

func TestGuard_GuardProjection(t *testing.T) {
	g := guard.New(
		guard.Event("foo", func(evt event.Of[test.FooEventData]) bool {
			return evt.Data().A == "foo"
		}),
		guard.Event("bar", func(evt event.Of[test.BarEventData]) bool {
			return evt.Data().A == "bar"
		}),
		guard.Any("foobar", func(evt event.Event) bool {
			return evt.Data().(test.FoobarEventData).A == 3
		}),
	)

	tests := map[event.Event]bool{
		event.New("foo", test.FooEventData{A: "foo"}).Any():   true,
		event.New("bar", test.BarEventData{A: "bar"}).Any():   true,
		event.New("foobar", test.FoobarEventData{A: 3}).Any(): true,
		event.New("foo", test.FooEventData{A: "bar"}).Any():   false,
		event.New("bar", test.BarEventData{A: "foo"}).Any():   false,
		event.New("foobar", test.FoobarEventData{A: 4}).Any(): false,
	}

	for evt, want := range tests {
		got := g.GuardProjection(evt)
		if got != want {
			t.Fatalf("GuardProjection() should return %v; got %v", want, got)
		}
	}
}
