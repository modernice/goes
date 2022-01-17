// Package mongostore provides a MongoDB event.Store.
package mongostore

import (
	"github.com/modernice/goes/backend/mongo"
	"github.com/modernice/goes/codec"
)

// New returns a MongoDB event.Store.
//
// Deprecated: Use github.com/modernice/goes/backend/mongo.NewEventStore instead.
func New(enc codec.Encoding[any], opts ...mongo.EventStoreOption) *mongo.EventStore {
	return mongo.NewEventStore(enc, opts...)
}
