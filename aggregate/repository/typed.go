package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
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

func Typed[Aggregate aggregate.Aggregate](r aggregate.Repository, makeFunc func(uuid.UUID) Aggregate) *TypedRepository[Aggregate] {
	return &TypedRepository[Aggregate]{repo: r, make: makeFunc}
}

func (r *TypedRepository[Aggregate]) Save(ctx context.Context, a Aggregate) error {
	return r.repo.Save(ctx, a)
}

func (r *TypedRepository[Aggregate]) Fetch(ctx context.Context, id uuid.UUID) (Aggregate, error) {
	out := r.make(id)
	return out, r.repo.Fetch(ctx, out)
}

func (r *TypedRepository[Aggregate]) FetchVersion(ctx context.Context, id uuid.UUID, v int) (Aggregate, error) {
	out := r.make(id)
	return out, r.repo.FetchVersion(ctx, out, v)
}

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

func (r *TypedRepository[Aggregate]) Use(ctx context.Context, id uuid.UUID, fn func(Aggregate) error) error {
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

func (r *TypedRepository[Aggregate]) Delete(ctx context.Context, a Aggregate) error {
	return r.repo.Delete(ctx, a)
}
