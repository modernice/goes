package cursor

import (
	"context"
	"errors"
	"fmt"

	"github.com/modernice/goes/aggregate"
)

var (
	// ErrClosed is returned by a Cursor when trying to advance it after it's
	// already been closed.
	ErrClosed = errors.New("cursor closed")
)

type cursor struct {
	aggregates []aggregate.Aggregate
	aggregate  aggregate.Aggregate
	pos        int
	err        error
	closed     chan struct{}
}

// New returns an in-memory Cursor filled with the provided aggregates.
func New(as ...aggregate.Aggregate) aggregate.Cursor {
	return &cursor{
		aggregates: as,
		closed:     make(chan struct{}),
	}
}

// All iterates over the Cursor cur and returns its aggregates. If a call to
// cur.Next causes an error, the already fetched aggregates and that error are
// returned. All automatically calls cur.Close(ctx) when done.
func All(ctx context.Context, cur aggregate.Cursor) (aggregates []aggregate.Aggregate, err error) {
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
		aggregates = append(aggregates, cur.Aggregate())
	}
	err = cur.Err()
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
	if len(c.aggregates) <= c.pos {
		return false
	}
	c.aggregate = c.aggregates[c.pos]
	c.pos++
	return true
}

func (c *cursor) Aggregate() aggregate.Aggregate {
	return c.aggregate
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
