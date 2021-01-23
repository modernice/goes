package xcursor

import (
	"context"
	"time"

	"github.com/modernice/goes/event"
)

type delayed struct {
	event.Cursor

	delay time.Duration
	err   error
}

// Delayed returns a Cursor c that adds an artificial delay to calls to c.Next.
func Delayed(cur event.Cursor, delay time.Duration) event.Cursor {
	return &delayed{
		Cursor: cur,
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

	s := c.Cursor.Next(ctx)
	c.err = c.Cursor.Err()
	return s
}

func (c *delayed) Err() error {
	return c.err
}
