// Package ref provides utilities for working with aggregate.Ref.
package ref

import (
	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
)

// Names returns the names of the given aggregates.
func Names(refs ...aggregate.Ref) []string {
	var names []string
	found := make(map[string]bool)
	for _, r := range refs {
		if r.Name == "" {
			continue
		}
		if found[r.Name] {
			continue
		}
		found[r.Name] = true
		names = append(names, r.Name)
	}
	return names
}

// IDs returns the UUIDs of the given aggregates.
func IDs(refs ...aggregate.Ref) []uuid.UUID {
	var ids []uuid.UUID
	found := make(map[uuid.UUID]bool)
	for _, t := range refs {
		if t.ID == uuid.Nil {
			continue
		}
		if found[t.ID] {
			continue
		}
		found[t.ID] = true
		ids = append(ids, t.ID)
	}
	return ids
}

// Aggregates returne the UUIDs of the given aggregates that have the given name.
func Aggregates(name string, refs ...aggregate.Ref) []uuid.UUID {
	var ids []uuid.UUID
	found := make(map[uuid.UUID]bool)
	for _, r := range refs {
		if r.Name != "" && r.Name == name {
			if found[r.ID] {
				continue
			}
			found[r.ID] = true
			ids = append(ids, r.ID)
		}
	}
	return ids
}
