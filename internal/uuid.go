package internal

import "github.com/google/uuid"

// NewUUID returns a version 7 UUID, panicking if generation fails.
func NewUUID() uuid.UUID {
	return uuid.Must(uuid.NewV7())
}
