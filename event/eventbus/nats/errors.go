package nats

import (
	"fmt"

	"github.com/modernice/goes/event"
)

// Error is an asynchronous error raised by the NATS event.Bus.
type Error struct {
	// Err is the underlying error.
	Err error
	// Event is the handled Event that caused the error.
	Event event.Event
}

func (err *Error) Error() string {
	return fmt.Sprintf("nats eventbus: %s", err.Err)
}

func (err *Error) Unwrap() error {
	return err.Err
}
