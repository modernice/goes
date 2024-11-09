package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/query"
	"github.com/modernice/goes/helper/pick"
	"golang.org/x/exp/slices"
)

// TypedRepository is a specialized version of an aggregate repository that
// deals with a specific type of aggregate. It provides methods for saving,
// fetching, querying and deleting aggregates of a certain type. The type of the
// aggregate this repository works with is determined by the factory function
// passed during the creation of the TypedRepository instance. It also provides
// access to the underlying generic repository and the factory function used to
// create new instances of the aggregate. The provided context is used for
// cancellation and timeout handling.
type TypedRepository[Aggregate aggregate.TypedAggregate] struct {
	repo aggregate.Repository
	make func(uuid.UUID) Aggregate
	name string
}

// Typed constructs a new TypedRepository for the provided aggregate.Repository
// and makeFunc. The makeFunc is used to create new instances of Aggregate. The
// returned TypedRepository only operates on Aggregates of the type created by
// makeFunc.
func Typed[Aggregate aggregate.TypedAggregate](r aggregate.Repository, makeFunc func(uuid.UUID) Aggregate) *TypedRepository[Aggregate] {
	return &TypedRepository[Aggregate]{
		repo: r,
		make: makeFunc,
		name: pick.AggregateName(makeFunc(uuid.Nil)),
	}
}

// NewOf creates a new instance of a TypedRepository for the specified Aggregate
// type. The provided aggregate.Repository and make function are used to
// construct the TypedRepository. The make function is used to create new
// instances of the Aggregate when needed.
func NewOf[Aggregate aggregate.TypedAggregate](r aggregate.Repository, makeFunc func(uuid.UUID) Aggregate) *TypedRepository[Aggregate] {
	return Typed(r, makeFunc)
}

// Repository returns the underlying aggregate.Repository of the
// TypedRepository. This allows direct access to the repository that stores and
// retrieves the aggregates.
func (r *TypedRepository[Aggregate]) Repository() aggregate.Repository {
	return r.repo
}

// NewFunc returns a function that creates a new instance of the Aggregate type.
// The returned function takes a UUID as an argument and returns an Aggregate.
// The UUID is used to identify the new Aggregate instance.
func (r *TypedRepository[Aggregate]) NewFunc() func(uuid.UUID) Aggregate {
	return r.make
}

// Save stores the provided aggregate into the repository. It takes a context
// for cancellation and deadline purposes and the aggregate to be stored.
// Returns an error if the saving process encounters any issues.
func (r *TypedRepository[Aggregate]) Save(ctx context.Context, a Aggregate) error {
	return r.repo.Save(ctx, a)
}

// Fetch retrieves the aggregate of the specified type and identifier from the
// repository. It returns the fetched aggregate and any error encountered during
// the process. If the aggregate does not exist, Fetch returns an error.
func (r *TypedRepository[Aggregate]) Fetch(ctx context.Context, id uuid.UUID) (Aggregate, error) {
	out := r.make(id)
	return out, r.repo.Fetch(ctx, out)
}

// FetchVersion retrieves a specific version of the aggregate identified by its
// UUID from the repository. The function returns an error if the requested
// version does not exist or if there is an issue fetching it from the
// repository.
func (r *TypedRepository[Aggregate]) FetchVersion(ctx context.Context, id uuid.UUID, version int) (Aggregate, error) {
	out := r.make(id)
	return out, r.repo.FetchVersion(ctx, out, version)
}

// Query returns a channel of Aggregates and a channel of errors found during
// the query execution. The Aggregates are retrieved from the underlying
// repository and are of the type that the TypedRepository is configured for.
// The returned Aggregates are created with the factory function provided during
// the construction of the TypedRepository. If an Aggregate is not of the
// expected type, it's skipped. The query execution can be cancelled through the
// provided context. The function also returns an error if there's an issue
// executing the query on the underlying repository.
func (r *TypedRepository[Aggregate]) Query(ctx context.Context, q aggregate.Query) (<-chan Aggregate, <-chan error, error) {
	qry := query.Expand(q)
	if !slices.Contains(qry.Names(), r.name) {
		qry.Q.Names = append(qry.Q.Names, r.name)
	}
	q = qry

	str, errs, err := r.repo.Query(ctx, q)
	if err != nil {
		return nil, nil, err
	}

	out := make(chan Aggregate)
	go func() {
		defer close(out)
		for his := range str {
			ref := his.Aggregate()

			a := r.make(ref.ID)

			if pick.AggregateName(a) != ref.Name {
				continue
			}

			his.Apply(a)

			select {
			case <-ctx.Done():
				return
			case out <- a:
			}
		}
	}()

	return out, errs, nil
}

// Refresh updates an aggregate to its latest state by fetching and applying any
// new events from the repository. This ensures the aggregate is up-to-date before
// persisting changes, which is important in concurrent scenarios (e.g. multiple
// long-running processes updating the same aggregate).
func (r *TypedRepository[Aggregate]) Refresh(ctx context.Context, a Aggregate) error {
	return r.repo.Fetch(ctx, a)
}

// Use retrieves an [Aggregate] with the provided UUID, applies the function fn
// to it, and then saves the [Aggregate] back into the repository. If fn returns
// an error, the [Aggregate] is not saved and the error is returned. The
// operation is performed in a context, and its execution may be cancelled if
// the context is closed.
func (r *TypedRepository[Aggregate]) Use(ctx context.Context, id uuid.UUID, fn func(Aggregate) error) error {
	a := r.make(id)
	return r.repo.Use(ctx, a, func() error { return fn(a) })
}

// Delete removes the specified Aggregate from the TypedRepository. It operates
// within the context passed to it. If any error occurs during the deletion, it
// is returned for handling.
func (r *TypedRepository[Aggregate]) Delete(ctx context.Context, a Aggregate) error {
	return r.repo.Delete(ctx, a)
}
