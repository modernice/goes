package xstream

import (
	"time"

	"github.com/modernice/goes/event"
)

// Delayed returns a Stream s that adds an artificial delay to calls to s.Next.
func Delayed[D any](delay time.Duration, events <-chan event.Of[D]) <-chan event.Of[D] {
	return DelayedFunc(func(event.Of[D]) time.Duration {
		return delay
	}, events)
}

// DelayedFunc returns a Stream s that adds an artificial delay to calls to s.Next.
func DelayedFunc[D any](fn func(prev event.Of[D]) time.Duration, events <-chan event.Of[D]) <-chan event.Of[D] {
	out := make(chan event.Of[D])
	go func() {
		defer close(out)
		var prev event.Of[D]
		for evt := range events {
			<-time.After(fn(prev))
			out <- evt
			prev = evt
		}
	}()
	return out
}
