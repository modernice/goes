package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/helper/pick"
)

// TypedRepository is a wrapper around aggregate.Repository that works with
// specific typed aggregates. It provides methods to save, fetch, query, and
// delete typed aggregates and utilizes an aggregate factory function to
// instantiate aggregates when needed.
type TypedRepository[Aggregate aggregate.TypedAggregate] struct {
	repo aggregate.Repository
	make func(uuid.UUID) Aggregate
}

// Typed returns a new TypedRepository for the provided aggregate.Repository and
// makeFunc. The makeFunc is used to create new instances of the typed Aggregate
// with a given UUID when fetching them from the underlying
// aggregate.Repository.
func Typed[Aggregate aggregate.TypedAggregate](r aggregate.Repository, makeFunc func(uuid.UUID) Aggregate) *TypedRepository[Aggregate] {
	return &TypedRepository[Aggregate]{repo: r, make: makeFunc}
}

// NewOf creates a new TypedRepository for the specified
// aggregate.TypedAggregate using the provided aggregate.Repository and factory
// function to instantiate Aggregates with a given UUID.
func NewOf[Aggregate aggregate.TypedAggregate](r aggregate.Repository, makeFunc func(uuid.UUID) Aggregate) *TypedRepository[Aggregate] {
	return Typed(r, makeFunc)
}

// Repository returns the underlying aggregate.Repository of the
// TypedRepository.
func (r *TypedRepository[Aggregate]) Repository() aggregate.Repository {
	return r.repo
}

// NewFunc returns the function that creates a new typed Aggregate with the
// given UUID. This function is used by the TypedRepository to instantiate
// Aggregates when fetching them from the underlying aggregate.Repository.
func (r *TypedRepository[Aggregate]) NewFunc() func(uuid.UUID) Aggregate {
	return r.make
}

// Save stores the given Aggregate in the repository and returns an error if the
// operation fails.
func (r *TypedRepository[Aggregate]) Save(ctx context.Context, a Aggregate) error {
	return r.repo.Save(ctx, a)
}

// Fetch retrieves the latest version of an Aggregate with the given UUID from
// the repository, returning the fetched Aggregate and any error encountered.
func (r *TypedRepository[Aggregate]) Fetch(ctx context.Context, id uuid.UUID) (Aggregate, error) {
	out := r.make(id)
	return out, r.repo.Fetch(ctx, out)
}

// FetchVersion retrieves an Aggregate with the specified ID and version from
// the TypedRepository, returning an error if it cannot be fetched.
func (r *TypedRepository[Aggregate]) FetchVersion(ctx context.Context, id uuid.UUID, version int) (Aggregate, error) {
	out := r.make(id)
	return out, r.repo.FetchVersion(ctx, out, version)
}

// Query retrieves Aggregates from the underlying aggregate.Repository that
// match the specified aggregate.Query. It returns channels for receiving the
// Aggregates and any errors encountered during the query, as well as an error
// if the query itself cannot be executed.
func (r *TypedRepository[Aggregate]) Query(ctx context.Context, q aggregate.Query) (<-chan Aggregate, <-chan error, error) {
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

// Use retrieves an Aggregate with the specified ID from the TypedRepository,
// applies the given function to it, and returns any error encountered during
// this process.
func (r *TypedRepository[Aggregate]) Use(ctx context.Context, id uuid.UUID, fn func(Aggregate) error) error {
	a := r.make(id)
	return r.repo.Use(ctx, a, func() error { return fn(a) })
}

// Delete removes the specified Aggregate from the repository.
func (r *TypedRepository[Aggregate]) Delete(ctx context.Context, a Aggregate) error {
	return r.repo.Delete(ctx, a)
}
