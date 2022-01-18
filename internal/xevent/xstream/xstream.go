package xstream

import (
	"time"

	"github.com/modernice/goes/event"
)

// Delayed returns a Stream s that adds an artificial delay to calls to s.Next.
func Delayed[D any](delay time.Duration, events <-chan event.EventOf[D]) <-chan event.EventOf[D] {
	return DelayedFunc(func(event.EventOf[D]) time.Duration {
		return delay
	}, events)
}

// DelayedFunc returns a Stream s that adds an artificial delay to calls to s.Next.
func DelayedFunc[D any](fn func(prev event.EventOf[D]) time.Duration, events <-chan event.EventOf[D]) <-chan event.EventOf[D] {
	out := make(chan event.EventOf[D])
	go func() {
		defer close(out)
		var prev event.EventOf[D]
		for evt := range events {
			<-time.After(fn(prev))
			out <- evt
			prev = evt
		}
	}()
	return out
}
