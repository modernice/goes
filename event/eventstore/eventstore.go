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

// WithBus returns a Store that publishes events through Bus b after inserting
// them into Store s.
func WithBus(s event.Store, b event.Bus) event.Store {
	return &storeWithBus{s, b}
}

func (s *storeWithBus) Insert(ctx context.Context, events ...event.Event) error {
	if err := s.Store.Insert(ctx, events...); err != nil {
		return err
	}
	if err := s.b.Publish(ctx, events...); err != nil {
		return fmt.Errorf("publish events: %w", err)
	}
	return nil
}
