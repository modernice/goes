package chanbus

import (
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventbus"
)

// New returns an in-memory event bus.
//
// Deprecated: Use github.com/modernice/goes/event/eventbus.New instead.
func New() event.Bus[any] {
	return eventbus.New[any]()
}
