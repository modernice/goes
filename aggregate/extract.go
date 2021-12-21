package aggregate

import "github.com/google/uuid"

// ExtractID returns the UUID of the given aggregate.
func ExtractID(a Aggregate) uuid.UUID {
	id, _, _ := a.Aggregate()
	return id
}

// ExtractName returns the name of the given aggregate.
func ExtractName(a Aggregate) string {
	_, name, _ := a.Aggregate()
	return name
}

// ExtractVersion returns the version of the given aggregate.
func ExtractVersion(a Aggregate) int {
	_, _, v := a.Aggregate()
	return v
}
