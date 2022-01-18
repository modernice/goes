package fanin

import (
	"context"

	"github.com/modernice/goes/helper/streams"
)

// Deprecated: Use streams.FanIn instead.
func Errors(errs ...<-chan error) (_ <-chan error, stop func()) {
	return streams.FanIn(errs...)
}

// Deprecated: Use streams.FanInContext instead.
func ErrorsContext(ctx context.Context, errs ...<-chan error) <-chan error {
	return streams.FanInContext(ctx, errs...)
}
