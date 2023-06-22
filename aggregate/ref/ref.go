package ref

import (
	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
)

// Names returns a slice of unique names of the given aggregate references. The
// order of names in the returned slice is the order in which they appear in the
// input.
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

// IDs returns a slice of unique UUIDs extracted from the provided
// [aggregate.Ref]s. It ignores any empty (uuid.Nil) IDs.
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

// Aggregates filters the provided [aggregate.Ref] slice by the given name, and
// returns a slice of unique [uuid.UUID] IDs of the matching aggregates.
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
