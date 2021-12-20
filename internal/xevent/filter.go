package xevent

import (
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/event"
)

// FilterAggregate filters events and returns only those that belong to the
// Aggregate a.
func FilterAggregate(events []event.Event, a aggregate.Aggregate) []event.Event {
	filtered := make([]event.Event, 0, len(events))
	for _, evt := range events {
		id, name, _ := evt.Aggregate()
		if name == a.AggregateName() && id == a.AggregateID() {
			filtered = append(filtered, evt)
		}
	}
	return filtered
}
