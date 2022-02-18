package repository

import (
	"context"
	"fmt"

	"github.com/modernice/goes"
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
type TypedRepository[Aggregate aggregate.AggregateOf[ID], ID goes.ID] struct {
	repo aggregate.RepositoryOf[ID]
	make func(ID) Aggregate
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
func Typed[ID goes.ID, Aggregate aggregate.AggregateOf[ID]](r aggregate.RepositoryOf[ID], makeFunc func(ID) Aggregate) *TypedRepository[Aggregate, ID] {
	return &TypedRepository[Aggregate, ID]{repo: r, make: makeFunc}
}

// Save implements aggregate.TypedRepository.Save.
func (r *TypedRepository[Aggregate, ID]) Save(ctx context.Context, a Aggregate) error {
	return r.repo.Save(ctx, a)
}

// Fetch implements aggregate.TypedRepository.Fetch.
func (r *TypedRepository[Aggregate, ID]) Fetch(ctx context.Context, id ID) (Aggregate, error) {
	out := r.make(id)
	return out, r.repo.Fetch(ctx, out)
}

// FetchVersion implements aggregate.TypedRepository.FetchVersion.
func (r *TypedRepository[Aggregate, ID]) FetchVersion(ctx context.Context, id ID, version int) (Aggregate, error) {
	out := r.make(id)
	return out, r.repo.FetchVersion(ctx, out, version)
}

// Query implements aggregate.TypedRepository.Query.
func (r *TypedRepository[Aggregate, ID]) Query(ctx context.Context, q aggregate.Query[ID]) (<-chan Aggregate, <-chan error, error) {
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
			//	name := pick.AggregateName[ID](r.make())
			//	q := query.New(query.AggregateName(name), ...)
			if pick.AggregateName[ID](a) != his.AggregateName() {
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
func (r *TypedRepository[Aggregate, ID]) Use(ctx context.Context, id ID, fn func(Aggregate) error) error {
	a, err := r.Fetch(ctx, id)
	if err != nil {
		return fmt.Errorf("fetch: %w [id=%v, type=%T]", err, id, a)
	}

	if err := fn(a); err != nil {
		return err
	}

	if err := r.repo.Save(ctx, a); err != nil {
		id, name, _ := a.Aggregate()
		return fmt.Errorf("save: %w [id=%v, name=%v, type=%T]", err, id, name, a)
	}

	return nil
}

// Delete implements aggregate.TypedRepository.Delete.
func (r *TypedRepository[Aggregate, ID]) Delete(ctx context.Context, a Aggregate) error {
	return r.repo.Delete(ctx, a)
}
