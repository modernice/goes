package stream

import "errors"

var (
	// ErrClosed is returned by a Stream when trying to read from it after it
	// has been closed.
	ErrClosed = errors.New("stream closed")
)
