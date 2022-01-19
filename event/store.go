package event

//go:generate mockgen -source=store.go -destination=./mocks/store.go

import (
	"context"

	"github.com/google/uuid"
	"github.com/modernice/goes/event/query/time"
	"github.com/modernice/goes/event/query/version"
)

const (
	// SortTime sorts events by time.
	SortTime = Sorting(iota)
	// SortAggregateName sorts events by their aggregate name.
	SortAggregateName
	// SortAggregateID sorts events by their aggregate id.
	SortAggregateID
	// SortAggregateVersion sorts events by their aggregate version.
	SortAggregateVersion
)

const (
	// SortAsc sorts events in ascending order.
	SortAsc = SortDirection(iota)
	// SortDesc sorts events in descending order.
	SortDesc
)

// A Store persists and queries events.
type Store interface {
	// Insert inserts Events into the store.
	Insert(context.Context, ...Event) error

	// Find fetches the Event with the specified UUID from the store.
	Find(context.Context, uuid.UUID) (Event, error)

	// Query queries the Store for Events that fit the given Query and returns a
	// channel of Events and a channel of errors.
	//
	// Example:
	//
	//	var store event.Store
	//	events, errs, err := store.Query(context.TODO(), query.New())
	//	// handle err
	//	err := streams.Walk(context.TODO(), func(evt event.Event) {
	//		log.Println(fmt.Sprintf("Queried Event: %v", evt))
	//	}, events, errs)
	//	// handle err
	Query(context.Context, Query) (<-chan Event, <-chan error, error)

	// Delete deletes Events from the Store.
	Delete(context.Context, ...Event) error
}

// A Query is used by Stores to query Events.
type Query interface {
	// Names returns the event names to query for.
	Names() []string

	// IDs returns the event ids to query for.
	IDs() []uuid.UUID

	// Times returns the time.Constraints for the query.
	Times() time.Constraints

	// AggregateNames returns the aggregate names to query for.
	AggregateNames() []string

	// AggregateIDs returns the aggregate ids to query for.
	AggregateIDs() []uuid.UUID

	// AggregateVersions returns the version.Constraints for the query.
	AggregateVersions() version.Constraints

	// Aggregates returns a list of specific Aggregates (name & ID pairs) to
	// query for. If an AggregateRef has a nil-UUID, every Aggregate with the
	// name of the tuple is queried.
	//
	// Example:
	//	id := uuid.New()
	//	q := query.New(query.Aggregate("foo", id), query.Aggregate("bar", uuid.Nil))
	//
	// The above Query q allows "foo" Aggregates with the UUID id and every "bar" Aggregate.
	Aggregates() []AggregateRef

	// Sorting returns the SortConfigs for the query.
	Sortings() []SortOptions
}

// AggregateRef is a reference to a specific aggregate, identified by its name
// and id.
type AggregateRef struct {
	Name string
	ID   uuid.UUID
}

// SortOptions defines the sorting behaviour of a Query.
type SortOptions struct {
	Sort Sorting
	Dir  SortDirection
}

// Sorting is a sorting.
type Sorting int

// SortDirection is a sorting direction.
type SortDirection int

// CompareSorting compares a and b based on the given sorting and returns
//	-1 if a < b
//	0 is a == b
//	1 if a > b
func CompareSorting[A, B any](s Sorting, a EventOf[A], b EventOf[B]) (cmp int8) {
	aid, aname, av := a.Aggregate()
	bid, bname, bv := b.Aggregate()

	switch s {
	case SortTime:
		return boolToCmp(a.Time().Before(b.Time()), a.Time().Equal(b.Time()))
	case SortAggregateName:
		return boolToCmp(
			aname < bname,
			aname == bname,
		)
	case SortAggregateID:
		return boolToCmp(
			aid.String() < bid.String(),
			aid == bid,
		)
	case SortAggregateVersion:
		return boolToCmp(
			av < bv,
			av == bv,
		)
	}
	return
}

// Compare compares a and b based on the given sorting and returns
//	-1 if a < b
//	0 is a == b
//	1 if a > b
func (s Sorting) Compare(a, b EventOf[any]) (cmp int8) {
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
