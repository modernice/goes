package aggregate

//go:generate mockgen -source=repository.go -destination=./mocks/repository.go

import (
	"context"

	"github.com/google/uuid"
	"github.com/modernice/goes"
	"github.com/modernice/goes/event/query/version"
	"github.com/modernice/goes/persistence/model"
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
type Repository = RepositoryOf[uuid.UUID]

// RepositoryOf is the aggregate repository. It saves and fetches aggregates to
// and from the underlying event store.
type RepositoryOf[ID goes.ID] interface {
	// Save inserts the changes of the aggregate into the event store.
	Save(ctx context.Context, a AggregateOf[ID]) error

	// Fetch fetches the events for the given Aggregate from the event store,
	// beginning from version a.AggregateVersion()+1 up to the latest version
	// for that Aggregate and applies them to a, so that a is in the latest
	// state. If the event store does not return any events, a stays untouched.
	Fetch(ctx context.Context, a AggregateOf[ID]) error

	// FetchVersion fetches the events for the given Aggregate from the event
	// store, beginning from version a.AggregateVersion()+1 up to v and applies
	// them to a, so that a is in the state of the time of the event with
	// version v. If the event store does not return any events, a stays
	// untouched.
	FetchVersion(ctx context.Context, a AggregateOf[ID], v int) error

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
	//		var a Aggregateggregate = newAggregate(app.AggregateName(), app.AggregateID())
	//		a.Apply(a)
	//	}
	Query(ctx context.Context, q Query[ID]) (<-chan HistoryOf[ID], <-chan error, error)

	// Use first fetches the Aggregate a from the event store, then calls fn(a)
	// and finally saves the aggregate changes. If fn returns a non-nil error,
	// the aggregate is not saved and the error is returned.
	Use(ctx context.Context, a AggregateOf[ID], fn func() error) error

	// Delete deletes an Aggregate by deleting its Events from the Event Store.
	Delete(ctx context.Context, a AggregateOf[ID]) error
}

// TypedRepository is a repository for a specific aggregate type.
// Use the github.com/modernnice/aggregate/repository.Typed function to create
// a TypedRepository.
//
//	func NewFoo(id uuid.UUID) *Foo { ... }
//
//	var repo aggregate.Repository
//	typed := repository.Typed(repo, NewFoo)
type TypedRepository[A interface {
	AggregateOf[ID]
	ModelID() ID
}, ID goes.ID] interface {
	model.TypedRepository[A, ID]

	// FetchVersion fetches all events for the given aggregate up to the given
	// version from the event store and applies them onto the aggregate.
	// FetchVersion fetches the given aggregate in the given version from the
	// event store.
	FetchVersion(ctx context.Context, id uuid.UUID, version int) (A, error)

	// Query queries the event store for aggregates and returns a channel of
	// aggregates and an error channel. If the query fails, Query returns nil
	// channels and an error.
	//
	// A query made by this repository will only ever return aggregates of this
	// repository's generic type, even if the query would normally return other
	// aggregates. Aggregates that cannot be casted to the generic type will be
	// simply discarded from the stream.
	//
	// The streams.Drain returns the query result as slice and a single error:
	//
	//	str, errs, err := r.Query(context.TODO(), query.New())
	//	// handle err
	//	res, err := streams.Drain(context.TODO(), str, errs)
	//	// handle err
	//	for _, a := range res {
	//		// a is your aggregate
	//	}
	Query(ctx context.Context, q Query[ID]) (<-chan A, <-chan error, error)
}

// Query is used by repositories to filter aggregates from the event store.
type Query[ID goes.ID] interface {
	// Names returns the aggregate names to query for.
	Names() []string

	// IDs returns the aggregate UUIDs to query for.
	IDs() []ID

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

type History = HistoryOf[uuid.UUID]

// A HistoryOf provides the event history of an aggregate. A History can be
// applied onto an aggregate to rebuild its current state.
type HistoryOf[ID goes.ID] interface {
	// AggregateName returns the name of the aggregate.
	AggregateName() string

	// AggregateID returns the id of the aggregate.
	AggregateID() ID

	// Apply applies the History on an Aggregate. Callers are responsible for
	// providing an Aggregate that can make use of the Events in the History.

	// Apply applies the history onto the aggregate to rebuild its current state.
	Apply(AggregateOf[ID])
}

func CompareSorting[ID goes.ID](s Sorting, a, b AggregateOf[ID]) (cmp int8) {
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

// Compare compares a and b and returns -1 if a < b, 0 if a == b or 1 if a > b.
func (s Sorting) Compare(a, b AggregateOf[goes.SID]) int8 {
	return CompareSorting(s, a, b)
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
