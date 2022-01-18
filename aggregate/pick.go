package aggregate

import (
	"github.com/google/uuid"
)

// Deprecated: Use PickID instead.
var ExtractID = PickID

// Deprecated: Use PickName instead.
var ExtractName = PickName

// Deprecated: Use PickVersion instead.
var ExtractVersion = PickVersion

// PickID returns the UUID of the given aggregate.
func PickID(a Aggregate) uuid.UUID {
	id, _, _ := a.Aggregate()
	return id
}

// PickName returns the name of the given aggregate.
func PickName(a Aggregate) string {
	_, name, _ := a.Aggregate()
	return name
}

// PickVersion returns the version of the given aggregate.
func PickVersion(a Aggregate) int {
	_, _, v := a.Aggregate()
	return v
}
