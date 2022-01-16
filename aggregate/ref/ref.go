// Package ref provides utilities for working with aggregate.Ref.
package ref

import (
	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
)

// Names extracts the Names of the given Tuples. Empty names are not included
// and duplicates are removed.
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

// IDs extracts the IDs of the given Tuples. Nil-UUIDs are not included and
// duplicates are removed.
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

// Aggregates extracts the IDs of the given Tuples that have the Name name.
// Nil-UUIDs are not included and duplicates are removed.
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
