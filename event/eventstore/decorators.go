package eventstore

import (
	"context"
	"fmt"

	"github.com/modernice/goes/event"
)

// WithBus decorates the given event store with the given event bus. Events are
// published over the bus after being inserted into the store.
//
// Insertion and publishing of events are not performed in a single transaction,
// which could cause a data loss if the bus fails to publish the events after
// insertion. To prevent this, check if there exists solution specific to the
// used event store implementation. For example, the MongoDB backend provides a
// service that subscribes to MongoDB Change Streams to publish events as they
// are inserted into the database.
//
// TODO(bounoable): Actually implement the MongoDB Change Streams service.
func WithBus(s event.Store, b event.Bus) event.Store {
	return &storeWithBus{
		Store: s,
		b:     b,
	}
}

type storeWithBus struct {
	event.Store
	b event.Bus
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
