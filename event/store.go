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

	// SortAsc sorts events in ascending order.
	SortAsc = SortDirection(iota)
	// SortDesc sorts events in descending order.
	SortDesc
)

// A Store persists and queries Events.
type Store interface {
	// Insert inserts Events into the store.
	Insert(context.Context, ...Event) error

	// Find fetches the Event with the specified UUID from the store.
	Find(context.Context, uuid.UUID) (Event, error)

	// Query queries the database for events filtered by Query q and returns a
	// Stream for those events.
	Query(context.Context, Query) (Stream, error)

	// Delete deletes the specified event from the store.
	Delete(context.Context, Event) error
}

// A Query is used by a Store to query Events and provides the filters for the
// query.
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

	// Sorting returns the SortConfigs for the query.
	Sortings() []SortOptions
}

// A Stream provides streaming over events.
type Stream interface {
	// Next should fetch the next Event from the underlying Store and return
	// true if the next call to Stream.Event would return that Event. If an
	// error occurred during Next, Stream.Err should return that error and
	// Stream.Event should return nil.
	Next(context.Context) bool

	// Event should return the current Event from the Stream or nil if
	// Stream.Next hasn't been called yet or because an error occurred during
	// Stream.Next.
	Event() Event

	// Err should return the error that occurred during the last call to
	// Stream.Next.
	Err() error

	// Close closes the stream.
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

// Compare compares a and b and returns -1 if a <= b, 0 is a == b or 1 if a > b.
func (s Sorting) Compare(a, b Event) (cmp int8) {
	switch s {
	case SortTime:
		return boolToCmp(a.Time().Before(b.Time()), a.Time().Equal(b.Time()))
	case SortAggregateName:
		return boolToCmp(
			a.AggregateName() <= b.AggregateName(),
			a.AggregateName() == b.AggregateName(),
		)
	case SortAggregateID:
		return boolToCmp(
			a.AggregateID().String() <= b.AggregateID().String(),
			a.AggregateID() == b.AggregateID(),
		)
	case SortAggregateVersion:
		return boolToCmp(
			a.AggregateVersion() <= b.AggregateVersion(),
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
