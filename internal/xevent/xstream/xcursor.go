package xstream

import (
	"context"
	"time"

	"github.com/modernice/goes/event"
)

type delayed struct {
	event.Stream

	delay time.Duration
	err   error
}

// Delayed returns a Stream s that adds an artificial delay to calls to s.Next.
func Delayed(s event.Stream, delay time.Duration) event.Stream {
	return &delayed{
		Stream: s,
		delay:  delay,
	}
}

func (c *delayed) Next(ctx context.Context) bool {
	timer := time.NewTimer(c.delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		c.err = ctx.Err()
		return false
	case <-timer.C:
	}

	s := c.Stream.Next(ctx)
	c.err = c.Stream.Err()
	return s
}

func (c *delayed) Err() error {
	return c.err
}
