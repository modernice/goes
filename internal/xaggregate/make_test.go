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
		if a.AggregateName() != "foo" {
			t.Errorf("aggregate should have name %q; got %q", "foo", a.AggregateName())
		}
		if a.AggregateID() == uuid.Nil {
			t.Errorf("aggregate id should be non-zero; got %v", a.AggregateID())
		}
		if a.AggregateVersion() != 0 {
			t.Errorf("aggregate version should be %d; got %d", 0, a.AggregateVersion())
		}

		if l := len(getAppliedEvents(a.AggregateID())); l != 0 {
			t.Errorf("aggregate should not have any events applied; got %d", l)
		}

		evt := event.New("foo", test.FooEventData{}, event.Aggregate(
			a.AggregateID(),
			a.AggregateName(),
			a.AggregateVersion()+1,
		))
		a.ApplyEvent(evt)

		test.AssertEqualEvents(t, getAppliedEvents(a.AggregateID()), []event.Event{evt})
	}
}

func TestName(t *testing.T) {
	as, _ := xaggregate.Make(3, xaggregate.Name("bar"))
	for _, a := range as {
		if a.AggregateName() != "bar" {
			t.Errorf("aggregate should have name %q; got %q", "bar", a.AggregateName())
		}
	}
}
