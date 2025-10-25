package aggregate

import (
	"context"

	"github.com/google/uuid"
	"github.com/modernice/goes/event/query/version"
	"github.com/modernice/goes/persistence/model"
)

const (
	// SortName sorts by aggregate name.
	SortName Sorting = iota
	// SortID sorts by aggregate id.
	SortID
	// SortVersion sorts by aggregate version.
	SortVersion

	// SortAsc orders ascending.
	SortAsc SortDirection = iota
	// SortDesc orders descending.
	SortDesc
)

// Repository stores and retrieves aggregates by working on their event streams.
// Implementations may also handle snapshots or concurrency concerns.
type Repository interface {
	Save(ctx context.Context, a Aggregate) error
	Fetch(ctx context.Context, a Aggregate) error
	FetchVersion(ctx context.Context, a Aggregate, v int) error
	Query(ctx context.Context, q Query) (<-chan History, <-chan error, error)
	Use(ctx context.Context, a Aggregate, fn func() error) error
	Delete(ctx context.Context, a Aggregate) error
}

// TypedAggregate combines a UUID model with Aggregate.
type TypedAggregate interface {
	model.Model[uuid.UUID]
	Aggregate
}

// TypedRepository exposes a Repository for a specific aggregate type.
type TypedRepository[A TypedAggregate] interface {
	model.Repository[A, uuid.UUID]
	FetchVersion(ctx context.Context, id uuid.UUID, version int) (A, error)
	Query(ctx context.Context, q Query) (<-chan A, <-chan error, error)
	Refresh(ctx context.Context, a A) error
}

// Query selects aggregates based on names, ids, versions and sort order.
type Query interface {
	Names() []string
	IDs() []uuid.UUID
	Versions() version.Constraints
	Sortings() []SortOptions
}

// SortOptions configure a sort field and direction.
type SortOptions struct {
	Sort Sorting
	Dir  SortDirection
}

// Sorting identifies a field used for ordering.
type Sorting int

// SortDirection chooses ascending or descending order.
type SortDirection int

// Compare compares two aggregates according to s.
func (s Sorting) Compare(a, b Aggregate) (cmp int8) {
	aid, aname, av := a.Aggregate()
	bid, bname, bv := b.Aggregate()
	switch s {
	case SortName:
		return boolToCmp(aname < bname, aname == bname)
	case SortID:
		return boolToCmp(aid.String() < bid.String(), aid == bid)
	case SortVersion:
		return boolToCmp(av < bv, av == bv)
	}
	return
}

// Bool interprets b according to dir.
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
