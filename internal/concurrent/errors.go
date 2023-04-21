package concurrent

import (
	"context"
	"sync"
)

// Errors returns an error channel and a function to push an error into the
// channel.
func Errors(ctx context.Context) (errs chan error, fail func(error)) {
	errs = make(chan error)
	return errs, Failer(ctx, errs)
}

// Failer is a function that returns a function to push an error into a channel.
// It takes a context and an error channel as arguments, and ensures that the
// error is pushed into the channel only if the context has not been cancelled.
func Failer(ctx context.Context, errs chan<- error) func(error) {
	var mux sync.Mutex
	var closed bool

	closeErrs := func() {
		close(errs)
		closed = true
	}

	go func() {
		<-ctx.Done()
		mux.Lock()
		defer mux.Unlock()
		if !closed {
			closeErrs()
		}
	}()

	return func(e error) {
		mux.Lock()
		defer mux.Unlock()

		if closed {
			return
		}

		select {
		case <-ctx.Done():
			closeErrs()
		case errs <- e:
		}
	}
}
