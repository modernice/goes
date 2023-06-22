package aggregate

import (
	"context"

	"github.com/google/uuid"
	"github.com/modernice/goes/event/query/version"
	"github.com/modernice/goes/persistence/model"
)

const (
	// SortName is a Sorting option that compares two Aggregates by their names in
	// ascending order. It is used in the Query interface for specifying the desired
	// sorting method when querying Aggregates.
	SortName = Sorting(iota)

	// SortID is a Sorting value used to sort Aggregates by their ID when comparing
	// them.
	SortID

	// SortVersion is a Sorting option that compares Aggregates based on their
	// version, ordering them by ascending or descending version numbers depending
	// on the SortDirection.
	SortVersion

	// SortAsc is a SortDirection that indicates sorting of aggregates in ascending
	// order based on the specified Sorting criteria.
	SortAsc = SortDirection(iota)

	// SortDesc is a [SortDirection] constant that specifies the descending order
	// for sorting Aggregates by their name, ID, or version.
	SortDesc
)

// Repository is an interface for persisting, fetching, querying, and deleting
// Aggregates. It provides methods for saving and fetching specific versions of
// an Aggregate, as well as querying multiple Aggregates based on a Query
// specification.
type Repository interface {
	// Save persists the given Aggregate to the Repository. Returns an error if the
	// operation fails.
	Save(ctx context.Context, a Aggregate) error

	// Fetch retrieves the latest state of the specified Aggregate from the
	// Repository and updates its state. It returns an error if the Aggregate cannot
	// be fetched.
	Fetch(ctx context.Context, a Aggregate) error

	// FetchVersion retrieves the specified version of an Aggregate from the
	// Repository and updates the provided Aggregate with the fetched data. It
	// returns an error if the version cannot be fetched or if the provided
	// Aggregate is not compatible with the fetched data.
	FetchVersion(ctx context.Context, a Aggregate, v int) error

	// Query executes a query on the Repository and returns channels for receiving
	// aggregate histories and errors. It takes a context and a Query as arguments.
	Query(ctx context.Context, q Query) (<-chan History, <-chan error, error)

	// Use applies the provided function fn to the given Aggregate a inside a
	// Repository, ensuring proper synchronization and error handling in the given
	// context. The function returns an error if any occurs during the execution of
	// fn or while interacting with the Repository.
	Use(ctx context.Context, a Aggregate, fn func() error) error

	// Delete removes the specified Aggregate from the Repository. It returns an
	// error if the deletion fails.
	Delete(ctx context.Context, a Aggregate) error
}

// TypedAggregate is an interface that composes the model.Model[uuid.UUID] and
// Aggregate interfaces, providing functionality for creating, fetching, and
// manipulating Aggregates with UUIDs as their identifier.
type TypedAggregate interface {
	model.Model[uuid.UUID]
	Aggregate
}

// TypedRepository is a specialized Repository for managing TypedAggregate
// instances, providing additional methods for fetching specific versions of an
// aggregate and querying with type safety. It extends the functionality of
// model.Repository, allowing for more fine-grained control over aggregate
// persistence and retrieval.
type TypedRepository[A TypedAggregate] interface {
	model.Repository[A, uuid.UUID]

	// FetchVersion retrieves an Aggregate with the specified UUID and version from
	// the TypedRepository. It returns the fetched Aggregate and an error if any
	// occurs during the fetch operation.
	FetchVersion(ctx context.Context, id uuid.UUID, version int) (A, error)

	// Query returns a channel of [TypedAggregate]s that match the specified query,
	// a channel for errors during the query execution, and an error if the query
	// can't be started. The returned channels must be fully consumed to avoid
	// goroutine leaks.
	Query(ctx context.Context, q Query) (<-chan A, <-chan error, error)
}

// Query represents a set of criteria for filtering and sorting Aggregates in a
// Repository. It defines which Aggregates should be included based on their
// names, IDs, and versions, as well as the order in which they should be
// sorted.
type Query interface {
	// Names returns a slice of aggregate names to be included in the query.
	Names() []string

	// IDs returns a slice of UUIDs that the Query is constrained to.
	IDs() []uuid.UUID

	// Versions returns the version constraints for a Query, which are used to
	// filter the queried Aggregates based on their version.
	Versions() version.Constraints

	// Sortings returns a slice of SortOptions that represent the sorting options
	// applied to a Query. The sorting options dictate the order in which Aggregates
	// are returned when executing the Query.
	Sortings() []SortOptions
}

// SortOptions is a struct that defines the sorting configuration for a query by
// specifying the sort field (Sorting) and sort direction (SortDirection). It is
// used in Query to determine the order of returned Aggregates.
type SortOptions struct {
	Sort Sorting
	Dir  SortDirection
}

// Sorting is a type that represents the sorting criteria for Aggregates in a
// Query. It provides methods to compare Aggregates based on their name, ID, or
// version. Use the constants SortName, SortID, and SortVersion to specify the
// desired sorting criteria.
type Sorting int

// SortDirection is an enumeration representing the direction of sorting
// (ascending or descending) when comparing Aggregates in a Query. It is used
// alongside Sorting to define the sorting criteria for querying Aggregates from
// a Repository.
type SortDirection int

// Compare compares two Aggregates based on the specified Sorting criteria. It
// returns a negative value if the first Aggregate comes before the second, a
// positive value if it comes after, and 0 if they are equal according to the
// chosen Sorting.
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

// Bool returns true if the SortDirection is SortAsc and the given boolean is
// true, or if the SortDirection is SortDesc and the given boolean is false.
// Otherwise, it returns false.
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
