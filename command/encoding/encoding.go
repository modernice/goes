package encoding

import (
	"errors"

	"github.com/modernice/goes/command"
)

var (
	// DefaultRegistry is the default Registry for Command Payloads. It defaults
	// to a GobEncoder.
	DefaultRegistry command.Registry = NewGobEncoder()

	// ErrUnregisteredCommand means command.Payload cannot be encoded/decoded
	// because it hasn't been registered yet.
	ErrUnregisteredCommand = errors.New("unregistered command")
)

// Register registers a Payload factory for Commands with the given name into the
// DefaultRegistry.
func Register(name string, new func() command.Payload) {
	DefaultRegistry.Register(name, new)
}
