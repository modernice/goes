package xstream

import (
	"time"

	"github.com/modernice/goes"
	"github.com/modernice/goes/event"
)

// Delayed returns a Stream s that adds an artificial delay to calls to s.Next.
func Delayed[D any, ID goes.ID](delay time.Duration, events <-chan event.Of[D, ID]) <-chan event.Of[D, ID] {
	return DelayedFunc(func(event.Of[D, ID]) time.Duration {
		return delay
	}, events)
}

// DelayedFunc returns a Stream s that adds an artificial delay to calls to s.Next.
func DelayedFunc[D any, ID goes.ID](fn func(prev event.Of[D, ID]) time.Duration, events <-chan event.Of[D, ID]) <-chan event.Of[D, ID] {
	out := make(chan event.Of[D, ID])
	go func() {
		defer close(out)
		var prev event.Of[D, ID]
		for evt := range events {
			<-time.After(fn(prev))
			out <- evt
			prev = evt
		}
	}()
	return out
}
