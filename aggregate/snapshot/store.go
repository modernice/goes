package snapshot

//go:generate mockgen -source=store.go -destination=./mocks/store.go Store

import (
	"context"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	aquery "github.com/modernice/goes/aggregate/query"
	"github.com/modernice/goes/event/query/time"
)

// Store is a database for aggregate snapshots.
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
	//	var store Store
	//	snaps, errs, err := store.Query(context.TODO(), query.New())
	//	// handle err
	//	err := streams.Walk(context.TODO(), func(snap Snapshot) {
	//		log.Println(fmt.Sprintf("Queried Snapshot: %v", snap))
	//		foo := newFoo(snap.AggregateID())
	//		err := Unmarshal(snap, foo)
	//		// handle err
	//	}, snaps, errs)
	//	// handle err
	Query(context.Context, Query) (<-chan Snapshot, <-chan error, error)

	// Delete deletes a Snapshot from the Store.
	Delete(context.Context, Snapshot) error
}

// Query is a query for snapshots.
type Query interface {
	aggregate.Query

	Times() time.Constraints
}

// Test tests the Snapshot s against the Query q and returns true if q should
// include s in its results. Test can be used by Store implementations
// to filter events based on the query.
func Test(q Query, s Snapshot) bool {
	if !aquery.Test[any](q, aggregate.New(
		s.AggregateName(),
		s.AggregateID(),
		aggregate.Version(s.AggregateVersion()),
	)) {
		return false
	}

	if times := q.Times(); times != nil {
		if exact := times.Exact(); len(exact) > 0 &&
			!timesContains(exact, s.Time()) {
			return false
		}
		if ranges := times.Ranges(); len(ranges) > 0 &&
			!testTimeRanges(ranges, s.Time()) {
			return false
		}
		if min := times.Min(); !min.IsZero() && !testMinTimes(min, s.Time()) {
			return false
		}
		if max := times.Max(); !max.IsZero() && !testMaxTimes(max, s.Time()) {
			return false
		}
	}

	return true
}
