package event

import (
	"context"

	"github.com/google/uuid"
	"github.com/modernice/goes/event/query/time"
	"github.com/modernice/goes/event/query/version"
)

// A Store persists and queries Events.
type Store interface {
	// Insert should insert evt into the Store.
	Insert(ctx context.Context, evt Event) error

	// Find should fetch the Event with the specified UUID from the Store.
	Find(ctx context.Context, id uuid.UUID) (Event, error)

	// Query should query Events according the Query q and return a Cursor for
	// those Events.
	Query(ctx context.Context, q Query) (Cursor, error)

	// Delete should remove the specified Event from the Store.
	Delete(ctx context.Context, evt Event) error
}

// A Query is used by a Store to query Events and provides the filters for the
// query.
type Query interface {
	// Names returns the event names to query for.
	Names() []string

	// IDs returns the event ids to query for.
	IDs() []uuid.UUID

	// Times returns the time.Constraints for the query.
	Times() time.Constraints

	// AggregateNames returns the aggregate names to query for.
	AggregateNames() []string

	// AggregateIDs returns the aggregate ids to query for.
	AggregateIDs() []uuid.UUID

	// AggregateVersions returns the version.Constraints for the query.
	AggregateVersions() version.Constraints
}

// A Cursor provides streaming over Events.
type Cursor interface {
	// Next should fetch the next Event from the underlying Store and return
	// true if the next call to Cursor.Event would return that Event. If an
	// error occurred during Next, Cursor.Err should return that error and
	// Cursor.Event should return nil.
	Next(context.Context) bool

	// Event should return the current Event from the Cursor or nil if
	// Cursor.Next hasn't been called yet or because an error occurred during
	// Cursor.Next.
	Event() Event

	// Err should return the error that occurred during the last call to
	// Cursor.Next.
	Err() error

	// Close should close the Cursor.
	Close(context.Context) error
}