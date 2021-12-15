package mongotest

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/event/eventstore/mongostore"
)

// NewStore returns a Store from the given Encoder and Options, but adds an
// Option that ensures a unique database name for every call to NewStore during
// the current process.
func NewStore(enc codec.Encoding, opts ...mongostore.Option) *mongostore.Store {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	id := hex.EncodeToString(b)

	return mongostore.New(enc, append(
		[]mongostore.Option{mongostore.Database(fmt.Sprintf("event_%s", id))},
		opts...,
	)...)
}
