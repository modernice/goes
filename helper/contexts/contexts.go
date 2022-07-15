package contexts

import (
	"context"
	"time"
)

// IgnoreCancel returns a new Context and CancelFunc for the Context.
// The returned Context is canceled when the provided Context is canceled,
// but with a delay that is at most `limit`.
func IgnoreCancel(ctx context.Context, limit time.Duration) (context.Context, context.CancelFunc) {
	out, cancel := context.WithCancel(context.Background())

	go func() {
		defer cancel()
		select {
		case <-out.Done():
			return
		case <-ctx.Done():
		}

		var limiter <-chan time.Time
		timer := time.NewTimer(limit)
		defer timer.Stop()
		limiter = timer.C

		select {
		case <-out.Done():
		case <-limiter:
		}
	}()

	return out, cancel
}
