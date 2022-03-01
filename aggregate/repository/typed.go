package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/helper/pick"
)

// TypedRepository implements aggregate.TypedRepository. Use Typed to create a
// typed repository from an underlying repository.
//
//	type Foo struct { *aggregate.Base }
//	func NewFoo(id uuid.UUID) *Foo { return &Foo{Base: aggregate.New("foo", id)} }
//
//	var repo aggregate.Repository
//	typed := repository.Typed(repo, NewFoo)
//
// Now you can use the typed repository like this:
//
//	foo, err := typed.Fetch(context.TODO(), uuid.UUID{...})
//
// For comparison, using an untyped repository:
//
//	var foo Foo
//	err := repo.Fetch(context.TODO(), &foo)
type TypedRepository[Aggregate aggregate.Aggregate] struct {
	repo aggregate.Repository
	make func(uuid.UUID) Aggregate
}

// Typed returns a TypedRepository for the given generic aggregate type. The
// provided makeFunc is used to initialize aggregates when querying from the
// event store.
//
//	type Foo struct { *aggregate.Base }
//	func NewFoo(id uuid.UUID) *Foo { return &Foo{Base: aggregate.New("foo", id)} }
//
//	var repo aggregate.Repository
//	typed := repository.Typed(repo, NewFoo)
func Typed[Aggregate aggregate.Aggregate](r aggregate.Repository, makeFunc func(uuid.UUID) Aggregate) *TypedRepository[Aggregate] {
	return &TypedRepository[Aggregate]{repo: r, make: makeFunc}
}

// Save implements aggregate.TypedRepository.Save.
func (r *TypedRepository[Aggregate]) Save(ctx context.Context, a Aggregate) error {
	return r.repo.Save(ctx, a)
}

// Fetch implements aggregate.TypedRepository.Fetch.
func (r *TypedRepository[Aggregate]) Fetch(ctx context.Context, id uuid.UUID) (Aggregate, error) {
	out := r.make(id)
	return out, r.repo.Fetch(ctx, out)
}

// FetchVersion implements aggregate.TypedRepository.FetchVersion.
func (r *TypedRepository[Aggregate]) FetchVersion(ctx context.Context, id uuid.UUID, version int) (Aggregate, error) {
	out := r.make(id)
	return out, r.repo.FetchVersion(ctx, out, version)
}

// Query implements aggregate.TypedRepository.Query.
func (r *TypedRepository[Aggregate]) Query(ctx context.Context, q aggregate.Query) (<-chan Aggregate, <-chan error, error) {
	str, errs, err := r.repo.Query(ctx, q)
	if err != nil {
		return nil, nil, err
	}

	out := make(chan Aggregate)
	go func() {
		defer close(out)
		for his := range str {
			a := r.make(his.AggregateID())

			// The query returned an aggregate that doesn't match the aggregate
			// type of the repository, so we discard it.
			//
			// TODO(bounoable): Maybe automatically add the aggregate name
			// filter to queries? We could theoretically determine the name of
			// the repository's aggregate using the provided makeFunc and then
			// add the query filter:
			//
			//	name := pick.AggregateName(r.make())
			//	q := query.New(query.AggregateName(name), ...)
			if pick.AggregateName(a) != his.AggregateName() {
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

// Use implements aggregate.TypedRepository.Use.
func (r *TypedRepository[Aggregate]) Use(ctx context.Context, id uuid.UUID, fn func(Aggregate) error) error {
	a := r.make(id)
	return r.repo.Use(ctx, a, func() error { return fn(a) })
}

// Delete implements aggregate.TypedRepository.Delete.
func (r *TypedRepository[Aggregate]) Delete(ctx context.Context, a Aggregate) error {
	return r.repo.Delete(ctx, a)
}
