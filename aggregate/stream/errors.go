package stream

import "errors"

var (
	// ErrClosed is returned by a Stream when trying to read from it or close it
	// after it has been closed.
	ErrClosed = errors.New("stream closed")

	// ErrNoFactory is returned when a Stream cannot find an Aggregate factory.
	ErrNoFactory = errors.New("no aggregate factory")
)
