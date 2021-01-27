package aggregate

//go:generate mockgen -source=repository.go -destination=./mocks/repository.go

import (
	"context"

	"github.com/google/uuid"
	"github.com/modernice/goes/event/query/version"
)

const (
	// SortName sorts aggregates by name.
	SortName = Sorting(iota)
	// SortID sorts aggregates by id.
	SortID
	// SortVersion sorts aggregates by version.
	SortVersion

	// SortAsc sorts aggregates in ascending order.
	SortAsc = SortDirection(iota)
	// SortDesc sorts aggregates in descending order.
	SortDesc
)

// Repository is the aggregate repository. It saves and fetches aggregates to
// and from the underlying event store.
type Repository interface {
	// Save inserts the changes of Aggregate a into the event store.
	Save(ctx context.Context, a Aggregate) error

	// Fetch fetches the events for the given Aggregate from the event store,
	// beginning from version a.AggregateVersion()+1 up to the latest version
	// for that Aggregate and applies them to a, so that a is in the latest
	// state. If the event store does not return any events, a stays untouched.
	Fetch(ctx context.Context, a Aggregate) error

	// FetchVersion fetches the events for the given Aggregate from the event
	// store, beginning from version a.AggregateVersion()+1 up to v and applies
	// them to a, so that a is in the state of the time of the event with
	// version v. If the event store does not return any events, a stays
	// untouched.
	FetchVersion(ctx context.Context, a Aggregate, v int) error

	// Delete deletes an Aggregate by deleting the events of the aggregate.
	Delete(ctx context.Context, a Aggregate) error

	// Query queries the event store for aggregates filtered by Query q and
	// returns a Stream for those aggregates.
	Query(ctx context.Context, q Query) (Stream, error)
}

// Query is used by Repositories to filter aggregates.
type Query interface {
	// Names returns the aggregate names to query for.
	Names() []string

	// IDs returns the aggregate UUIDs to query for.
	IDs() []uuid.UUID

	// Versions returns the version constraints for the query.
	Versions() version.Constraints

	// Sortings returns the SortConfig for the query.
	Sortings() []SortOptions
}

// A Stream iterates over aggregates.
type Stream interface {
	// Next should fetch the next Aggregate from the underlying Store and return
	// true if the next call to Stream.Aggregate would return that Aggregate. If
	// an error occurred during Next, Stream.Err should return that error and
	// Stream.Aggregate should return nil.
	Next(context.Context) bool

	// Aggregate should return the current Aggregate from the Stream or nil if
	// Stream.Next hasn't been called yet or because an error occurred during
	// Stream.Next.
	Aggregate() Aggregate

	// Err should return the error that occurred during the last call to
	// Stream.Next.
	Err() error

	// Close should close the Stream.
	Close(context.Context) error
}

// SortOptions defines the sorting behaviour for a Query.
type SortOptions struct {
	Sort Sorting
	Dir  SortDirection
}

// Sorting is a sorting.
type Sorting int

// SortDirection is a sorting direction.
type SortDirection int

// Compare compares a and b and returns -1 if a < b, 0 if a == b or 1 if a > b.
func (s Sorting) Compare(a, b Aggregate) (cmp int8) {
	switch s {
	case SortName:
		return boolToCmp(
			a.AggregateName() < b.AggregateName(),
			a.AggregateName() == b.AggregateName(),
		)
	case SortID:
		return boolToCmp(
			a.AggregateID().String() < b.AggregateID().String(),
			a.AggregateID() == b.AggregateID(),
		)
	case SortVersion:
		return boolToCmp(
			a.AggregateVersion() < b.AggregateVersion(),
			a.AggregateVersion() == b.AggregateVersion(),
		)
	}
	return
}

// Bool returns either b if dir=SortAsc or !b if dir=SortDesc.
func (dir SortDirection) Bool(b bool) bool {
	if dir == SortDesc {
		return !b
	}
	return b
}

func boolToCmp(b, same bool) int8 {
	if same {
		return 0
	}
	if b {
		return -1
	}
	return 1
}
