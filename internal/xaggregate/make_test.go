package xaggregate_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/test"
	"github.com/modernice/goes/internal/xaggregate"
)

func TestMake(t *testing.T) {
	as, getAppliedEvents := xaggregate.Make(10)
	if len(as) != 10 {
		t.Errorf("Make(%d) should return %d aggregates; got %d", 10, 10, len(as))
	}

	for _, a := range as {
		id, name, v := a.Aggregate()

		if name != "foo" {
			t.Errorf("aggregate should have name %q; got %q", "foo", name)
		}
		if id == uuid.Nil {
			t.Errorf("aggregate id should be non-zero; got %v", id)
		}
		if v != 0 {
			t.Errorf("aggregate version should be %d; got %d", 0, v)
		}

		if l := len(getAppliedEvents(id)); l != 0 {
			t.Errorf("aggregate should not have any events applied; got %d", l)
		}

		evt := event.New[any]("foo", test.FooEventData{}, event.Aggregate[any](id, name, v+1))
		a.ApplyEvent(evt)

		test.AssertEqualEvents(t, getAppliedEvents(id), []event.Event{evt})
	}
}

func TestName(t *testing.T) {
	as, _ := xaggregate.Make(3, xaggregate.Name("bar"))
	for _, a := range as {
		_, name, _ := a.Aggregate()
		if name != "bar" {
			t.Errorf("aggregate should have name %q; got %q", "bar", name)
		}
	}
}
