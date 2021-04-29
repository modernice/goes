package unique

import "github.com/google/uuid"

// UUID returns only unique UUIDs.
func UUID(ids ...uuid.UUID) []uuid.UUID {
	if ids == nil {
		return nil
	}

	c := make(map[uuid.UUID]bool)
	uniq := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if !c[id] {
			uniq = append(uniq, id)
			c[id] = true
		}
	}

	return uniq
}
