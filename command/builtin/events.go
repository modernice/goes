package builtin

import "github.com/google/uuid"

// AggregateDeleted is published when an aggregate has been deleted.
const AggregateDeleted = "goes.command.aggregate.deleted"

// AggregateDeletedData is the event data for the AggregateDeleted event.
type AggregateDeletedData struct {
	// Name is the name of the deleted aggregate.
	Name string

	// ID is the UUID of the deleted aggregate.
	ID uuid.UUID

	// Version is the version of the deleted aggregate.
	Version int
}
