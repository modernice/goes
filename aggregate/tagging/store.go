package tagging

//go:generate mockgen -source=store.go -destination=./mock_tagging/store.go

import (
	"context"

	"github.com/google/uuid"
)

type Store interface {
	// Update updates the tags of an aggregate.
	Update(_ context.Context, aggregateName string, aggregateID uuid.UUID, tags []string) error

	// Tags returns the tags of an aggregate.
	Tags(context.Context, string, uuid.UUID) ([]string, error)

	// TaggesWith returns the Aggregates that have the provided tags.
	TaggedWith(context.Context, ...string) ([]Aggregate, error)
}

type Aggregate struct {
	Name string
	ID   uuid.UUID
}
