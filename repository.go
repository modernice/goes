package goes

import (
	"context"
)

// TypedRepository is a generic repository that can be used to define
// repositories for any kind of model.
//
// aggregate.TypedRepository extends this interface.
type TypedRepository[ID, T any] interface {
	// Save saves the given model to the database.
	Save(ctx context.Context, a T) error

	// Fetch fetches the model with the given id from the database.
	Fetch(ctx context.Context, id ID) (T, error)

	// Use first fetches the given aggregate from the event store, then calls
	// the provided function with the aggregate as the argument and finally
	// save aggregate back to the event store. If fn returns a non-nil error,
	// the aggregate is not saved and the error is returned.

	// Use first fetches the model from the database, then passes the model to
	// the provided function fn and finally saves the model back to the database.
	Use(ctx context.Context, id ID, fn func(T) error) error

	// Delete deletes the given model from the database.
	Delete(ctx context.Context, a T) error
}
