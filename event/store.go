package event

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
	// SortAggregateName sorts events by aggregate name.
	SortAggregateName
	// SortAggregateID sorts events by aggregate id.
	SortAggregateID
	// SortAggregateVersion sorts events by aggregate version.
	SortAggregateVersion
)

const (
	// SortAsc sorts events in ascending order.
	SortAsc = SortDirection(iota)
	// SortDesc sorts events in descending order.
	SortDesc
)

// A Store provides persistence for events.
type Store interface {
	// Insert inserts events into the store.
	Insert(context.Context, ...Event) error

	// Find fetches the given event from the store.
	Find(context.Context, uuid.UUID) (Event, error)

	// Query queries the store for events and returns two channels – one for the
	// returned events and one for any asynchronous errors that occur during the
	// query.
	//
	//	var store event.Store
	//	events, errs, err := store.Query(context.TODO(), query.New(...))
	//	// handle err
	//	err := streams.Walk(context.TODO(), func(evt event.Event) {
	//		log.Println(fmt.Sprintf("Queried event: %s", evt.Name()))
	//	}, events, errs)
	//	// handle err
	Query(context.Context, Query) (<-chan Event, <-chan error, error)

	// Delete deletes events from the store.
	Delete(context.Context, ...Event) error
}

// A Query can be used to query events from an event store. Each of the query's
// methods that return a non-nil filter are considered when filtering events.
// Different (non-nil) filters must all be fulfilled by an event to be included
// in the result. Within a single filter that allows multiple values, the event
// must match at least one of the values.
type Query interface {
	// Names returns the event names to query for.
	Names() []string

	// IDs returns the event ids to query for.
	IDs() []uuid.UUID

	// Times returns the event time constraints for the query.
	Times() time.Constraints

	// AggregateNames returns the aggregate names to query for.
	AggregateNames() []string

	// AggregateIDs returns the aggregate ids to query for.
	AggregateIDs() []uuid.UUID

	// AggregateVersions returns the event version constraints for the query.
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
	//
	// Advantage of using this filter instead of using the `AggregateNames()`
	// and `AggregateIDs()` filters is that this filter allows to query multiple
	// specific aggregates with different names.
	Aggregates() []AggregateRef

	// Sorting returns the sorting options for the query. Events are sorted as
	// they would be by calling SortMulti().
	Sortings() []SortOptions
}

// AggregateRef is a reference to a specific aggregate, identified by its name
// and id.
type AggregateRef struct {
	Name string
	ID   uuid.UUID
}

// SortOptions defines the sorting of a query.
type SortOptions struct {
	Sort Sorting
	Dir  SortDirection
}

// Sorting is a sorting.
type Sorting int

// SortDirection is a sorting direction, either ascending or descending.
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

// String returns the string representation of the aggregate:
//	"NAME(ID)"
func (ref AggregateRef) String() string {
	return fmt.Sprintf("%s(%s)", ref.Name, ref.ID)
}

var refStringRE = regexp.MustCompile(`([^()]+?)(\([a-z0-9-]+?\))`)

// Parse parses the string-representation of an AggregateRef into ref.
// Parse accepts values that are returned by AggregateRef.String().
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
