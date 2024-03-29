package xaggregate

import (
	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
)

// Map takes a slice of aggregates and returns a map that provides quick access
// to those aggregates by their UUID.
func Map(as []aggregate.Aggregate) map[uuid.UUID]aggregate.Aggregate {
	m := make(map[uuid.UUID]aggregate.Aggregate, len(as))
	for _, a := range as {
		id, _, _ := a.Aggregate()
		m[id] = a
	}
	return m
}
