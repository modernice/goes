package fanin

import (
	"context"
	"sync"
)

// Errors accepts multiple error channels and returns a single error channel.
// When the returned stop function is called or every input channel is closed,
// the returned error channel is closed.
//
// If len(errs) == 0, Errors returns a closed error channel.
//
// Multiple calls to stop have no effect.
func Errors(errs ...<-chan error) (_ <-chan error, stop func()) {
	stopped := make(chan struct{})
	var once sync.Once
	stop = func() { once.Do(func() { close(stopped) }) }

	out := make(chan error)

	var wg sync.WaitGroup
	wg.Add(len(errs))
	for _, errs := range errs {
		go func(errs <-chan error) {
			defer wg.Done()
			for {
				select {
				case <-stopped:
					return
				case err, ok := <-errs:
					if !ok {
						return
					}
					select {
					case <-stopped:
						return
					case out <- err:
					}
				}
			}
		}(errs)
	}

	go func() {
		wg.Wait()
		close(out)
	}()

	return out, stop
}

// Errors accepts a Context and multiple error channels and returns a single
// error channel. When ctx is canceled or every input channel is closed, the
// returned error channel is closed.
//
// If len(errs) == 0, Errors returns a closed error channel.
func ErrorsContext(ctx context.Context, errs ...<-chan error) <-chan error {
	out, stop := Errors(errs...)
	go func() {
		<-ctx.Done()
		stop()
	}()
	return out
}
