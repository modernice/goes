package eventbus

import (
	"context"
	"fmt"

	"github.com/modernice/goes/event"
)

type busWithStore struct {
	bus   event.Bus
	store event.Store
}

// WithStore returns a Bus that inserts Events into the provided Store before
// publishing.
func WithStore(bus event.Bus, store event.Store) event.Bus {
	return &busWithStore{bus, store}
}

func (bus *busWithStore) Publish(ctx context.Context, events ...event.Event) error {
	if err := bus.store.Insert(ctx, events...); err != nil {
		return fmt.Errorf("store: %w", err)
	}

	if err := bus.bus.Publish(ctx, events...); err != nil {
		return fmt.Errorf("publish: %w", err)
	}

	return nil
}

func (bus *busWithStore) Subscribe(ctx context.Context, names ...string) (<-chan event.Event, error) {
	return bus.bus.Subscribe(ctx, names...)
}
