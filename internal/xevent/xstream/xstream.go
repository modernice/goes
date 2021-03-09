package xstream

import (
	"time"

	"github.com/modernice/goes/event"
)

// Delayed returns a Stream s that adds an artificial delay to calls to s.Next.
func Delayed(delay time.Duration, events <-chan event.Event) <-chan event.Event {
	return DelayedFunc(func(event.Event) time.Duration {
		return delay
	}, events)
}

// DelayedFunc returns a Stream s that adds an artificial delay to calls to s.Next.
func DelayedFunc(fn func(prev event.Event) time.Duration, events <-chan event.Event) <-chan event.Event {
	out := make(chan event.Event)
	go func() {
		defer close(out)
		var prev event.Event
		for evt := range events {
			<-time.After(fn(prev))
			out <- evt
			prev = evt
		}
	}()
	return out
}
