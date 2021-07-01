package tuple

import (
	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
)

// Names extracts the Names of the given Tuples. Empty names are not included
// and duplicates are removed.
func Names(tuples ...aggregate.Tuple) []string {
	var names []string
	found := make(map[string]bool)
	for _, t := range tuples {
		if t.Name == "" {
			continue
		}
		if found[t.Name] {
			continue
		}
		found[t.Name] = true
		names = append(names, t.Name)
	}
	return names
}

// IDs extracts the IDs of the given Tuples. Nil-UUIDs are not included and
// duplicates are removed.
func IDs(tuples ...aggregate.Tuple) []uuid.UUID {
	var ids []uuid.UUID
	found := make(map[uuid.UUID]bool)
	for _, t := range tuples {
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
func Aggregates(name string, tuples ...aggregate.Tuple) []uuid.UUID {
	var ids []uuid.UUID
	found := make(map[uuid.UUID]bool)
	for _, t := range tuples {
		if t.Name != "" && t.Name == name {
			if found[t.ID] {
				continue
			}
			found[t.ID] = true
			ids = append(ids, t.ID)
		}
	}
	return ids
}
