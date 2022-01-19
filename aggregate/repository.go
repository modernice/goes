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

	// Query queries the Event Store for Aggregates and returns a channel of
	// Histories and an error channel. If the query fails, Query returns nil
	// channels and an error.
	//
	// A History can be applied on an Aggregate to reconstruct its state from
	// the History.
	//
	// The Drain function can be used to get the result of the stream as slice
	// and a single error:
	//
	//	res, errs, err := r.Query(context.TODO(), query.New())
	//	// handle err
	//	appliers, err := stream.Drain(context.TODO(), res, errs)
	//	// handle err
	//	for _, app := range appliers {
	//		// Initialize your Aggregate.
	//		var a Aggregate = newAggregate(app.AggregateName(), app.AggregateID())
	//		a.Apply(a)
	//	}
	Query(ctx context.Context, q Query) (<-chan History, <-chan error, error)

	// Use first fetches the Aggregate a from the event store, then calls fn(a)
	// and finally saves the aggregate changes. If fn returns a non-nil error,
	// the aggregate is not saved and the error is returned.
	Use(ctx context.Context, a Aggregate, fn func() error) error

	// Delete deletes an Aggregate by deleting its Events from the Event Store.
	Delete(ctx context.Context, a Aggregate) error
}

type TypedRepository[A Aggregate] interface {
	Save(ctx context.Context, a A) error

	Fetch(ctx context.Context, id uuid.UUID) (A, error)

	FetchVersion(ctx context.Context, id uuid.UUID, v int) (A, error)

	Query(ctx context.Context, q Query) (<-chan A, <-chan error, error)

	Use(ctx context.Context, id uuid.UUID, fn func(A) error) error

	// Delete deletes an Aggregate by deleting its Events from the Event Store.
	Delete(ctx context.Context, a A) error
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

// SortOptions defines the sorting behaviour for a Query.
type SortOptions struct {
	Sort Sorting
	Dir  SortDirection
}

// Sorting is a sorting.
type Sorting int

// SortDirection is a sorting direction.
type SortDirection int

// A History provides the event history of an aggregate. A History can be
// applied onto an aggregate to rebuild its current state.
type History interface {
	// AggregateName returns the name of the aggregate.
	AggregateName() string

	// AggregateID returns the UUID of the aggregate.
	AggregateID() uuid.UUID

	// Apply applies the History on an Aggregate. Callers are responsible for
	// providing an Aggregate that can make use of the Events in the History.

	// Apply applies the history onto the aggregate to rebuild its current state.
	Apply(Aggregate)
}

// Compare compares a and b and returns -1 if a < b, 0 if a == b or 1 if a > b.
func (s Sorting) Compare(a, b Aggregate) (cmp int8) {
	aid, aname, av := a.Aggregate()
	bid, bname, bv := b.Aggregate()

	switch s {
	case SortName:
		return boolToCmp(
			aname < bname,
			aname == bname,
		)
	case SortID:
		return boolToCmp(
			aid.String() < bid.String(),
			aid == bid,
		)
	case SortVersion:
		return boolToCmp(
			av < bv,
			av == bv,
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
