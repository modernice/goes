package xevent_test

import (
	"sort"
	"testing"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/test"
	"github.com/modernice/goes/internal/xevent"
)

func TestShuffle(t *testing.T) {
	a := aggregate.New("foo", uuid.New())
	events := xevent.Make("foo", test.FooEventData{}, 100, xevent.ForAggregate(a))
	sorted := make([]event.Event[any], len(events))
	copy(sorted, events)
	sort.Slice(sorted, func(i, j int) bool {
		return event.PickAggregateVersion(sorted[i]) < event.PickAggregateVersion(sorted[j])
	})
	test.AssertEqualEvents(t, sorted, events)

	xevent.Shuffle(events)
	test.AssertEqualEvents(t, sorted, events)

	events = xevent.Shuffle(events)

	if test.EqualEvents(events, sorted) {
		t.Errorf(
			"shuffled events should not equal unshuffled events\n\n"+
				"sorted: %#v\n\nshuffled: %#v\n\n",
			sorted,
			events,
		)
	}
}
