package xstream

import (
	"time"

	"github.com/modernice/goes/event"
)

// Delayed returns a Stream s that adds an artificial delay to calls to s.Next.
func Delayed[D any](delay time.Duration, events <-chan event.Event[D]) <-chan event.Event[D] {
	return DelayedFunc(func(event.Event[D]) time.Duration {
		return delay
	}, events)
}

// DelayedFunc returns a Stream s that adds an artificial delay to calls to s.Next.
func DelayedFunc[D any](fn func(prev event.Event[D]) time.Duration, events <-chan event.Event[D]) <-chan event.Event[D] {
	out := make(chan event.Event[D])
	go func() {
		defer close(out)
		var prev event.Event[D]
		for evt := range events {
			<-time.After(fn(prev))
			out <- evt
			prev = evt
		}
	}()
	return out
}
