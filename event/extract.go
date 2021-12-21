package event

import "github.com/google/uuid"

// ExtractAggregateID returns the ExtractAggregateID of the given event.
func ExtractAggregateID(evt Event) uuid.UUID {
	id, _, _ := evt.Aggregate()
	return id
}

// ExtractAggregateName returns the ExtractAggregateName of the given event.
func ExtractAggregateName(evt Event) string {
	_, name, _ := evt.Aggregate()
	return name
}

// ExtractAggregateVersion returns the ExtractAggregateVersion of the given event.
func ExtractAggregateVersion(evt Event) int {
	_, _, v := evt.Aggregate()
	return v
}
