package xstream

import (
	"context"
	"time"

	"github.com/modernice/goes/event"
)

type delayed struct {
	event.Stream

	delayFunc func(event.Event) time.Duration
	prev      event.Event
	err       error
}

// Delayed returns a Stream s that adds an artificial delay to calls to s.Next.
func Delayed(s event.Stream, delay time.Duration) event.Stream {
	return DelayedFunc(s, func(event.Event) time.Duration {
		return delay
	})
}

// DelayedFunc returns a Stream s that adds an artificial delay to calls to s.Next.
func DelayedFunc(s event.Stream, fn func(prev event.Event) time.Duration) event.Stream {
	return &delayed{
		Stream:    s,
		delayFunc: fn,
	}
}

func (c *delayed) Next(ctx context.Context) bool {
	timer := time.NewTimer(c.delayFunc(c.prev))
	defer timer.Stop()

	select {
	case <-ctx.Done():
		c.err = ctx.Err()
		return false
	case <-timer.C:
	}

	s := c.Stream.Next(ctx)
	c.err = c.Stream.Err()
	c.prev = c.Stream.Event()
	return s
}

func (c *delayed) Err() error {
	return c.err
}
