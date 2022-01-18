package eventstore

import (
	"context"
	"fmt"

	"github.com/modernice/goes/event"
)

type storeWithBus[D any] struct {
	event.Store[D]

	b event.Bus[D]
}

// WithBus returns a Store that publishes Events over the given Bus after
// inserting them into the provided Store.
func WithBus[D any](s event.Store[D], b event.Bus[D]) event.Store[D] {
	return &storeWithBus[D]{
		Store: s,
		b:     b,
	}
}

func (s *storeWithBus[D]) Insert(ctx context.Context, events ...event.EventOf[D]) error {
	if err := s.Store.Insert(ctx, events...); err != nil {
		return err
	}

	if err := s.b.Publish(ctx, events...); err != nil {
		return fmt.Errorf("publish Events: %w", err)
	}

	return nil
}
