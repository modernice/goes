package cursor

import (
	"context"
	"errors"
	"fmt"

	"github.com/modernice/goes/event"
)

var (
	// ErrClosed is returned by a Cursor when trying to advance it after it's
	// already been closed.
	ErrClosed = errors.New("cursor closed")
)

type cursor struct {
	events []event.Event
	event  event.Event
	pos    int
	err    error
	closed chan struct{}
}

// New returns an in-memory Cursor filled with the provided events.
func New(events ...event.Event) event.Cursor {
	return &cursor{
		events: events,
		closed: make(chan struct{}),
	}
}

// All iterates over the Cursor cur and returns its Events. If a call to
// cur.Next causes an error, the already fetched Events and that error are
// returned. All automatically calls cur.Close(ctx) when done.
func All(ctx context.Context, cur event.Cursor) (events []event.Event, err error) {
	defer func() {
		if cerr := cur.Close(ctx); cerr != nil {
			if err != nil {
				err = fmt.Errorf("[0] %w\n[1] %s", err, cerr)
				return
			}
			err = cerr
		}
	}()
	for cur.Next(ctx) {
		events = append(events, cur.Event())
	}
	if err = cur.Err(); err != nil {
		return
	}
	return
}

func (c *cursor) Next(ctx context.Context) bool {
	select {
	case <-c.closed:
		c.err = ErrClosed
		return false
	default:
		c.err = nil
	}

	if len(c.events) <= c.pos {
		return false
	}
	c.event = c.events[c.pos]
	c.pos++
	return true
}

func (c *cursor) Event() event.Event {
	return c.event
}

func (c *cursor) Err() error {
	return c.err
}

func (c *cursor) Close(context.Context) error {
	select {
	case <-c.closed:
	default:
		close(c.closed)
	}
	return nil
}
