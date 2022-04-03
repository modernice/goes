package event

//go:generate mockgen -source=store.go -destination=./mocks/store.go

import (
	"context"
	"fmt"
	"regexp"

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
	// Insert inserts events into the store.
	Insert(context.Context, ...Event) error

	// Find fetches the event with the specified UUID from the store.
	Find(context.Context, uuid.UUID) (Event, error)

	// Query queries the Store for events that fit the given Query and returns a
	// channel of events and a channel of errors.
	//
	// Example:
	//
	//	var store event.Store
	//	events, errs, err := store.Query(context.TODO(), query.New())
	//	// handle err
	//	err := streams.Walk(context.TODO(), func(evt event.Event) {
	//		log.Println(fmt.Sprintf("Queried event: %v", evt))
	//	}, events, errs)
	//	// handle err
	Query(context.Context, Query) (<-chan Event, <-chan error, error)

	// Delete deletes events from the Store.
	Delete(context.Context, ...Event) error
}

// A Query is used by Stores to query events.
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

	// Aggregates returns a list of specific aggregates (name & id pairs) to
	// query for. If an AggregateRef has a nil-UUID, every Aggregate with the
	// given name is queried.
	//
	// Example:
	//	id := uuid.New()
	//	q := query.New(query.Aggregate("foo", id), query.Aggregate("bar", uuid.Nil))
	//
	// The above query allows the "foo" aggregate with the specified id and
	// every "bar" aggregate. Events that do not fulfill any of these two
	// constraints will not be returned.
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
func CompareSorting[A, B any](s Sorting, a Of[A], b Of[B]) (cmp int8) {
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
func (s Sorting) Compare(a, b Of[any]) (cmp int8) {
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

var zeroRef AggregateRef

// IsZero returns whether the ref has an empty name and a nil-UUID.
func (ref AggregateRef) IsZero() bool { return ref == zeroRef }

// String returns the string representation of the aggregate: NAME(ID)
func (ref AggregateRef) String() string {
	return fmt.Sprintf("%s(%s)", ref.Name, ref.ID)
}

var refStringRE = regexp.MustCompile(`([^()]+?)(\([a-z0-9-]+?\))`)

// Parse parses the string-representation of an aggregateRef into ref.
// Parse accepts values that are returned by ref.String().
func (ref *AggregateRef) Parse(v string) error {
	matches := refStringRE.FindStringSubmatch(v)
	if len(matches) != 3 {
		return fmt.Errorf("invalid ref string: %q", v)
	}

	id, err := uuid.Parse(matches[2])
	if err != nil {
		return fmt.Errorf("invalid ref string: %q", v)
	}
	ref.Name = matches[1]
	ref.ID = id

	return nil
}
