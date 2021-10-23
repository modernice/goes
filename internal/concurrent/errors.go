package concurrent

import "context"

// Errors returns an error channel and a function to push an error into the
// channel.
func Errors(ctx context.Context) (chan error, func(error)) {
	out := make(chan error)
	return out, func(e error) {
		select {
		case <-ctx.Done():
		case out <- e:
		}
	}
}
