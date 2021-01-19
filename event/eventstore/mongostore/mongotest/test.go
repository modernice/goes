package mongotest

import (
	"fmt"
	"sync/atomic"

	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventstore/mongostore"
)

var (
	dbCounter int32
)

// NewStore returns a Store from the given Encoder and Options, but adds an
// Option that ensures a unique database name for every call to NewStore during
// the current process.
func NewStore(enc event.Encoder, opts ...mongostore.Option) *mongostore.Store {
	n := atomic.AddInt32(&dbCounter, 1)
	return mongostore.New(enc, append(
		opts,
		mongostore.Database(fmt.Sprintf("event_%d", n-1)))...,
	)
}
