package mongotest

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/modernice/goes/backend/mongo"
	"github.com/modernice/goes/codec"
)

// NewEventStore returns a Store from the given Encoder and Options, but adds an
// Option that ensures a unique database name for every call to NewEventStore
// during the current process.
func NewEventStore(enc codec.Encoding, opts ...mongo.EventStoreOption) *mongo.EventStore {
	return mongo.NewEventStore(enc, append(
		[]mongo.EventStoreOption{mongo.Database(UniqueName("event_"))},
		opts...,
	)...)
}

// UniqueName generates a unique string by appending a random hexadecimal
// string to the provided prefix. The generated string is intended for use as a
// unique identifier in database names. If there's an error in reading from the
// random source, the function will panic.
func UniqueName(prefix string) string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	id := hex.EncodeToString(b)
	return fmt.Sprintf("%s%s", prefix, id)
}
