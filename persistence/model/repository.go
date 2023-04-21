package model

import (
	"context"
	"errors"
	"fmt"
)

// ErrNotFound is returned by repositories when a model cannot be found.
var ErrNotFound = errors.New("model not found")

// ID represents a unique, comparable identifier for a model in the repository.
// It extends the comparable and fmt.Stringer interfaces.
type ID interface {
	comparable
	fmt.Stringer
}

// Model is an entity with a unique, comparable identifier.
type Model[T ID] interface {
	ModelID() T
}

// Repository is a generic type that can be used to define repositories for
// any kind of model.
//
// aggregate.TypedRepository extends this interface.
type Repository[M Model[IDT], IDT ID] interface {
	// Save saves the given model to the database.
	Save(ctx context.Context, a M) error

	// Fetch fetches the model with the given id from the database. If no model
	// with the given id can be found, an error that unwraps to ErrNotFound
	// should be returned.
	Fetch(ctx context.Context, id IDT) (M, error)

	// Use first fetches the given model from the event store, then calls
	// the provided function with the model as the argument and finally
	// save model back to the event store. If fn returns a non-nil error,
	// the model is not saved and the error is returned.

	// Use first fetches the model from the database, then passes the model to
	// the provided function fn and finally saves the model back to the database.
	Use(ctx context.Context, id IDT, fn func(M) error) error

	// Delete deletes the given model from the database.
	Delete(ctx context.Context, a M) error
}
