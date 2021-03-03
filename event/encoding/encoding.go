package encoding

import "github.com/modernice/goes/event"

// DefaultRegistry is the default Event Registy. It defaults to a GobEncoder.
var DefaultRegistry event.Registry = NewGobEncoder()

// Register registers a Data factory for Events with the given name into the
// DefaultRegistry.
func Register(name string, new func() event.Data) {
	DefaultRegistry.Register(name, new)
}
