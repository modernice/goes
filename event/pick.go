package event

import "github.com/google/uuid"

// Deprecated: Use PickAggregateID instead.
var ExtractAggregatecVersionID = PickAggregateID

// Deprecated: Use PickAggregateName instead.
var ExtractAggregateName = PickAggregateName

// Deprecated: Use PickAggregateVersion instead.
var ExtractAggregatecVersion = PickAggregateName

// PickAggregateID returns the AggregateID of the given event.
func PickAggregateID(evt Event) uuid.UUID {
	id, _, _ := evt.Aggregate()
	return id
}

// PickAggregateName returns the AggregateName of the given event.
func PickAggregateName(evt Event) string {
	_, name, _ := evt.Aggregate()
	return name
}

// PickAggregateVersion returns the AggregateVersion of the given event.
func PickAggregateVersion(evt Event) int {
	_, _, v := evt.Aggregate()
	return v
}
