package xevent

import (
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/event"
)

// FilterAggregate filters events and returns only those that belong to the
// Aggregate a.
func FilterAggregate(events []event.Event[any], a aggregate.Aggregate) []event.Event[any] {
	filtered := make([]event.Event[any], 0, len(events))
	for _, evt := range events {
		id, name, _ := evt.Aggregate()
		aid, aname, _ := a.Aggregate()
		if name == aname && id == aid {
			filtered = append(filtered, evt)
		}
	}
	return filtered
}
