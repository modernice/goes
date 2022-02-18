package xevent

import (
	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/event"
)

// FilterAggregate filters events and returns only those that belong to the
// Aggregate a.
func FilterAggregate(events []event.Of[any, uuid.UUID], a aggregate.Aggregate) []event.Of[any, uuid.UUID] {
	filtered := make([]event.Of[any, uuid.UUID], 0, len(events))
	for _, evt := range events {
		id, name, _ := evt.Aggregate()
		aid, aname, _ := a.Aggregate()
		if name == aname && id == aid {
			filtered = append(filtered, evt)
		}
	}
	return filtered
}
