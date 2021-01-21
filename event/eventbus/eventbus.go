package eventbus

import (
	"context"
	"fmt"

	"github.com/modernice/goes/event"
)

type busWithStore struct {
	event.Bus
	s event.Store
}

// WithStore returns a Bus that inserts events into Store s before publishing
// them through Bus b.
func WithStore(b event.Bus, s event.Store) event.Bus {
	return &busWithStore{b, s}
}

func (bus *busWithStore) Publish(ctx context.Context, events ...event.Event) error {
	if err := bus.s.Insert(ctx, events...); err != nil {
		return fmt.Errorf("store: %w", err)
	}

	if err := bus.Bus.Publish(ctx, events...); err != nil {
		return fmt.Errorf("publish: %w", err)
	}

	return nil
}
