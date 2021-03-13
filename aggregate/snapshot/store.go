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

	// Query queries the Store for Snapshots that fit the given Query and
	// returns a channel of Snapshots and a channel of errors.
	//
	// Example:
	//
	//	var store snapshot.Store
	//	snaps, errs, err := store.Query(context.TODO(), query.New())
	//	// handle err
	//	err := snapshot.Walk(context.TODO(), func(snap snapshot.Snapshot) {
	//		log.Println(fmt.Sprintf("Queried Snapshot: %v", snap))
	//		foo := newFoo(snap.AggregateID())
	//		err := snapshot.Unmarshal(snap, foo)
	//		// handle err
	//	}, snaps, errs)
	//	// handle err
	Query(context.Context, aggregate.Query) (<-chan Snapshot, <-chan error, error)

	// Delete deletes a Snapshot from the Store.
	Delete(context.Context, Snapshot) error
}
