package streams

import (
	"context"
	"sync"
)

// FanIn returns a single receive-only channel from multiple receive-only
// channels. When the returned stop function is called or every input channel
// is closed, the returned channel is closed.
//
// If len(in) == 0, FanIn returns a closed channel.
//
// Multiple calls to stop have no effect.
func FanIn[T any](in ...<-chan T) (_ <-chan T, stop func()) {
	stopped := make(chan struct{})
	var once sync.Once
	stop = func() { once.Do(func() { close(stopped) }) }

	out := make(chan T)

	var wg sync.WaitGroup
	wg.Add(len(in))
	for _, in := range in {
		go func(in <-chan T) {
			defer wg.Done()
			for {
				select {
				case <-stopped:
					return
				case v, ok := <-in:
					if !ok {
						return
					}
					select {
					case <-stopped:
						return
					case out <- v:
					}
				}
			}
		}(in)
	}

	go func() {
		wg.Wait()
		close(out)
	}()

	return out, stop
}

// FanInContext returns a single receive-only channel from multiple receive-only
// channels. When the provided ctx is canceled or every input channel is closed,
// the returned channel is closed.
//
// If len(in) == 0, FanInContext returns a closed channel.
func FanInContext[T any](ctx context.Context, in ...<-chan T) <-chan T {
	out, stop := FanIn(in...)
	go func() {
		<-ctx.Done()
		stop()
	}()
	return out
}

// FanInAll returns FanIn(in...) but without the stop function.
func FanInAll[T any](in ...<-chan T) <-chan T {
	out, _ := FanIn(in...)
	return out
}
