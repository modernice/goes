package eventstore

import (
	"context"
	"fmt"

	"github.com/modernice/goes"
	"github.com/modernice/goes/event"
)

type storeWithBus[ID goes.ID] struct {
	event.Store[ID]

	b event.Bus[ID]
}

// WithBus returns a Store that publishes Events over the given Bus after
// inserting them into the provided Store.
func WithBus[ID goes.ID](s event.Store[ID], b event.Bus[ID]) event.Store[ID] {
	return &storeWithBus[ID]{
		Store: s,
		b:     b,
	}
}

func (s *storeWithBus[ID]) Insert(ctx context.Context, events ...event.Of[any, ID]) error {
	if err := s.Store.Insert(ctx, events...); err != nil {
		return err
	}

	if err := s.b.Publish(ctx, events...); err != nil {
		return fmt.Errorf("publish events: %w", err)
	}

	return nil
}
