package event

import (
	"context"
	"fmt"
	"regexp"

	"github.com/google/uuid"
	"github.com/modernice/goes/event/query/time"
	"github.com/modernice/goes/event/query/version"
)

// #region sortings
const (
	// SortTime is a Sorting option that sorts events by their timestamp, with
	// earlier events coming before later events.
	SortTime = Sorting(iota)

	// SortAggregateName is a Sorting value that sorts events by their aggregate
	// name in a Query. Events with the same aggregate name are considered equal
	// when sorting with this value.
	SortAggregateName

	// SortAggregateID is a Sorting option that sorts events based on the
	// lexicographical order of their aggregate ID.
	SortAggregateID

	// SortAggregateVersion is a Sorting option that sorts events based on their
	// aggregate version, with lower versions coming first.
	SortAggregateVersion
)

const (
	// SortAsc is a SortDirection constant that represents sorting in ascending
	// order. Use it with Sorting and SortOptions to specify the desired sort
	// direction when querying events from an event store.
	SortAsc = SortDirection(iota)

	// SortDesc is a SortDirection that indicates the order of sorting should be in
	// descending order when comparing values.
	SortDesc
)

// #endregion sortings

// #region store
//
// Store is an interface that provides methods for managing Event storage,
// including insertion, retrieval, querying, and deletion of events.
// Implementations should handle the storage and retrieval of events according
// to the provided Query constraints, such as filtering by aggregate names, IDs,
// and versions.
type Store interface {
	// Insert inserts the provided Events into the Store. Returns an error if any
	// Event could not be inserted.
	Insert(context.Context, ...Event) error

	// Find retrieves the Event with the specified UUID from the Store. It returns
	// an error if the Event could not be found or if there was an issue accessing
	// the Store.
	Find(context.Context, uuid.UUID) (Event, error)

	// Query searches for Events in the Store that match the provided Query and
	// returns two channels: one for the found Events and another for errors that
	// may occur during the search. An error is returned if the search cannot be
	// started.
	Query(context.Context, Query) (<-chan Event, <-chan error, error)

	// Delete removes the specified Events from the Store. It returns an error if
	// any of the deletions fail.
	Delete(context.Context, ...Event) error
}

// #endregion store

// #region query
//
// Query is an interface that represents a set of criteria for filtering and
// sorting events when querying an event store. It provides methods to access
// constraints on event names, aggregate IDs, time ranges, aggregate names,
// aggregate versions, and custom sorting options.
type Query interface {
	// Names returns a slice of event names included in the Query.
	Names() []string

	// IDs returns a slice of UUIDs that the Query should match. The returned events
	// will have their EventID equal to one of the UUIDs in the slice.
	IDs() []uuid.UUID

	// Times returns the time constraints of the query, specifying the desired range
	// of event timestamps to be included in the result set.
	Times() time.Constraints

	// AggregateNames returns a slice of aggregate names that the Query is filtering
	// for.
	AggregateNames() []string

	// AggregateIDs returns a slice of UUIDs representing the aggregate IDs that the
	// Query is constrained to.
	AggregateIDs() []uuid.UUID

	// AggregateVersions returns the version constraints of the queried Aggregates
	// as a version.Constraints value.
	AggregateVersions() version.Constraints

	// Aggregates returns a slice of AggregateRef, representing the aggregate
	// references that match the query.
	Aggregates() []AggregateRef

	// Sortings returns a slice of SortOptions specifying the sorting criteria for
	// the query results. The events in the result set will be sorted according to
	// the provided sorting options in the order they appear in the slice.
	Sortings() []SortOptions
}

// #endregion query

// AggregateRef represents a reference to an aggregate with a specific Name and
// ID. It provides methods to check if it's a zero value, retrieve aggregate
// information, split the Name and ID, and parse a string into an AggregateRef.
type AggregateRef struct {
	Name string
	ID   uuid.UUID
}

// SortOptions is a configuration struct used to specify the sorting criteria
// and direction for event queries. It contains a Sorting field to determine the
// sorting attribute (such as time, aggregate name, aggregate ID, or aggregate
// version) and a SortDirection field to indicate the sorting order (ascending
// or descending).
type SortOptions struct {
	Sort Sorting
	Dir  SortDirection
}

// Sorting is an enumeration of the possible ways to sort Events when querying a
// Store. Supported sort options include sorting by time, aggregate name,
// aggregate ID, and aggregate version.
type Sorting int

// SortDirection determines the order of sorting in a query. It can be either
// ascending (SortAsc) or descending (SortDesc).
type SortDirection int

// CompareSorting compares two events a and b using the specified Sorting s. It
// returns -1 if a is less than b, 0 if they are equal, or 1 if a is greater
// than b. The comparison is based on the time, aggregate name, aggregate ID, or
// aggregate version, depending on the provided Sorting s.
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

// Compare returns the comparison result of two events, a and b, based on the
// provided Sorting value s. The comparison result is -1 if a < b, 1 if a > b,
// or 0 if a == b.
func (s Sorting) Compare(a, b Of[any]) (cmp int8) {
	return CompareSorting(s, a, b)
}

// Bool returns true if the given bool b matches the SortDirection, and false
// otherwise. If SortDirection is SortDesc, the result is the negation of b.
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

// IsZero reports whether the AggregateRef is a zero value, meaning it has an
// empty Name and a zero UUID.
func (ref AggregateRef) IsZero() bool { return ref == zeroRef }

// Aggregate returns the ID, name, and version of the AggregateRef. The returned
// version is always -1 as AggregateRef does not store version information.
func (ref AggregateRef) Aggregate() (uuid.UUID, string, int) {
	return ref.ID, ref.Name, -1
}

// Split returns the ID and Name of the AggregateRef.
func (ref AggregateRef) Split() (uuid.UUID, string) {
	return ref.ID, ref.Name
}

// String returns a string representation of the AggregateRef in the format
// "Name(UUID)".
func (ref AggregateRef) String() string {
	return fmt.Sprintf("%s(%s)", ref.Name, ref.ID)
}

var refStringRE = regexp.MustCompile(`([^()]+?)(\([a-z0-9-]+?\))`)

// Parse parses the given string representation of an AggregateRef and sets the
// Name and ID fields of the receiver. The input string should be in the format
// "Name(ID)" where Name is a non-empty string and ID is a valid UUID. Returns
// an error if the input string is invalid or cannot be parsed.
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
