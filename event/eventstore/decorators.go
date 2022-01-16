package eventstore

import (
	"context"
	"fmt"

	"github.com/modernice/goes/event"
)

type storeWithBus struct {
	event.Store

	b event.Bus
}

// WithBus returns a Store that publishes Events over the given Bus after
// inserting them into the provided Store.
func WithBus(s event.Store, b event.Bus) event.Store {
	return &storeWithBus{
		Store: s,
		b:     b,
	}
}

func (s *storeWithBus) Insert(ctx context.Context, events ...event.Event[any]) error {
	if err := s.Store.Insert(ctx, events...); err != nil {
		return err
	}

	if err := s.b.Publish(ctx, events...); err != nil {
		return fmt.Errorf("publish Events: %w", err)
	}

	return nil
}
