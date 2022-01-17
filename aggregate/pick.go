package aggregate

import "github.com/google/uuid"

// Deprecated: Use PickID instead.
var ExtractID = PickID[any]

// Deprecated: Use PickName instead.
var ExtractName = PickName[any]

// Deprecated: Use PickVersion instead.
var ExtractVersion = PickVersion[any]

// PickID returns the UUID of the given aggregate.
func PickID[D any](a Aggregate[D]) uuid.UUID {
	id, _, _ := a.Aggregate()
	return id
}

// PickName returns the name of the given aggregate.
func PickName[D any](a Aggregate[D]) string {
	_, name, _ := a.Aggregate()
	return name
}

// PickVersion returns the version of the given aggregate.
func PickVersion[D any](a Aggregate[D]) int {
	_, _, v := a.Aggregate()
	return v
}
