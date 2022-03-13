package commonpb

import "github.com/google/uuid"

// NewUUID converts a uuid.UUID to a *UUID.
func NewUUID(id uuid.UUID) *UUID {
	return &UUID{Bytes: id[:]}
}

// AsUUID converts the *UUID to a uuid.UUID.
func (id *UUID) AsUUID() uuid.UUID {
	b := id.GetBytes()
	if len(b) != 16 {
		return uuid.Nil
	}
	var out uuid.UUID
	copy(out[:], b)
	return out
}
