package event

import "github.com/google/uuid"

// Deprecated: Use PickAggregateID instead.
var ExtractAggregateVersionID = PickAggregateID[any]

// Deprecated: Use PickAggregateName instead.
var ExtractAggregateName = PickAggregateName[any]

// Deprecated: Use PickAggregateVersion instead.
var ExtractAggregatecVersion = PickAggregateName[any]

// PickAggregateID returns the AggregateID of the given event.
func PickAggregateID[D any](evt Event[D]) uuid.UUID {
	id, _, _ := evt.Aggregate()
	return id
}

// PickAggregateName returns the AggregateName of the given event.
func PickAggregateName[D any](evt Event[D]) string {
	_, name, _ := evt.Aggregate()
	return name
}

// PickAggregateVersion returns the AggregateVersion of the given event.
func PickAggregateVersion[D any](evt Event[D]) int {
	_, _, v := evt.Aggregate()
	return v
}
