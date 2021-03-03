package encoding

import (
	"errors"

	"github.com/modernice/goes/event"
)

var (
	// DefaultRegistry is the default Event Registy. It defaults to a GobEncoder.
	DefaultRegistry event.Registry = NewGobEncoder()

	// ErrUnregisteredEvent means event.Data cannot be encoded/decoded because
	// it hasn't been registered yet.
	ErrUnregisteredEvent = errors.New("unregistered event")
)

// Register registers a Data factory for Events with the given name into the
// DefaultRegistry.
func Register(name string, new func() event.Data) {
	DefaultRegistry.Register(name, new)
}
