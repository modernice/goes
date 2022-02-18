package mongotest

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/google/uuid"
	"github.com/modernice/goes/backend/mongo"
	"github.com/modernice/goes/codec"
)

// NewEventStore returns a Store from the given Encoder and Options, but adds an
// Option that ensures a unique database name for every call to NewEventStore
// during the current process.
func NewEventStore(enc codec.Encoding, opts ...mongo.EventStoreOption) *mongo.EventStore[uuid.UUID] {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	id := hex.EncodeToString(b)

	return mongo.NewEventStore[uuid.UUID](enc, append(
		[]mongo.EventStoreOption{mongo.Database(fmt.Sprintf("event_%s", id))},
		opts...,
	)...)
}
