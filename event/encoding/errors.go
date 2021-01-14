package encoding

import "errors"

var (
	// ErrUnregisteredEvent means event.Data cannot be encoded/decoded because
	// it hasn't been registered yet.
	ErrUnregisteredEvent = errors.New("unregistered event")
)
