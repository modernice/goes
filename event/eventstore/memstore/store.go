package memstore

import (
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventstore"
)

// New returns an in-memory event store.
//
// Deprecated: Use github.com/modernice/goes/event/eventstore.New instead.
func New() event.Store {
	return eventstore.New()
}
