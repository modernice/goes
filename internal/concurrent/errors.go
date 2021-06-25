package concurrent

import "context"

// Errors returns an error channel and a function to push an error into that
// channel. The function returns when either the error is received from the
// channel or ctx is canceled.
func Errors(ctx context.Context) (chan error, func(err error)) {
	errs := make(chan error)
	return errs, func(err error) {
		select {
		case <-ctx.Done():
		case errs <- err:
		}
	}
}
