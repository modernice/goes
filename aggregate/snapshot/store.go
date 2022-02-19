package snapshot

//go:generate mockgen -source=store.go -destination=./mocks/store.go Store

import (
	"context"

	"github.com/modernice/goes"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/event/query/time"
)

// Store is a database for aggregate snapshots.
type Store[ID goes.ID] interface {
	// Save saves the given Snapshot into the Store.
	Save(context.Context, Snapshot[ID]) error

	// Latest returns the latest Snapshot for the Aggregate with the given name
	// and UUID.
	Latest(context.Context, string, ID) (Snapshot[ID], error)

	// Version returns the Snapshot with the given version for the Aggregate
	// with the given name and UUID. Implementations should return an error if
	// the specified Snapshot does not exist in the Store.
	Version(context.Context, string, ID, int) (Snapshot[ID], error)

	// Limit returns the latest Snapshot that has a version equal to or lower
	// than the given version. Implementations should return an error if no
	// such Snapshot can be found.
	Limit(context.Context, string, ID, int) (Snapshot[ID], error)

	// Query queries the Store for Snapshots that fit the given Query and
	// returns a channel of Snapshots and a channel of errors.
	//
	// Example:
	//
	//	var store snapshot.Store
	//	snaps, errs, err := store.Query(context.TODO(), query.New())
	//	// handle err
	//	err := streams.Walk(context.TODO(), func(snap snapshot.Snapshot) {
	//		log.Println(fmt.Sprintf("Queried Snapshot: %v", snap))
	//		foo := newFoo(snap.AggregateID())
	//		err := snapshot.Unmarshal(snap, foo)
	//		// handle err
	//	}, snaps, errs)
	//	// handle err
	Query(context.Context, Query[ID]) (<-chan Snapshot[ID], <-chan error, error)

	// Delete deletes a Snapshot from the Store.
	Delete(context.Context, Snapshot[ID]) error
}

// Query is a query for snapshots.
type Query[ID goes.ID] interface {
	aggregate.Query[ID]

	Times() time.Constraints
}
