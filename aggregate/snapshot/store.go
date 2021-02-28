package snapshot

//go:generate mockgen -source=store.go -destination=./mocks/store.go

import (
	"context"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
)

// Store is a database for Snapshots.
type Store interface {
	// Save saves the given Snapshot into the Store.
	Save(context.Context, Snapshot) error

	// Latest returns the latest Snapshot for the Aggregate with the given name
	// and UUID.
	Latest(context.Context, string, uuid.UUID) (Snapshot, error)

	// Version returns the Snapshot with the given version for the Aggregate
	// with the given name and UUID. Implementations should return an error if
	// the specified Snapshot does not exist in the Store.
	Version(context.Context, string, uuid.UUID, int) (Snapshot, error)

	// Limit returns the latest Snapshot that has a version equal to or lower
	// than the given version. Implementations should return an error if no
	// such Snapshot can be found.
	Limit(context.Context, string, uuid.UUID, int) (Snapshot, error)

	// Query queries the Store for Snapshots and returns a Stream for those
	// Snapshots.
	Query(context.Context, aggregate.Query) (Stream, error)

	// Delete deletes a Snapshot from the Store.
	Delete(context.Context, Snapshot) error
}

// A Stream iterates over Snapshots.
type Stream interface {
	// Next fetches the next Snapshot from the Stream and returns whether the
	// fetch was successful. When Next returns false, Err should return the
	// error that made the fetch fail. Otherwise Snapshot returns the current
	// Snapshot.
	Next(context.Context) bool

	// Snapshot returns the current Snapshot from the Stream.
	Snapshot() Snapshot

	// Err returns the current error from the Stream.
	Err() error

	// Close closes the Stream.
	Close(context.Context) error
}
