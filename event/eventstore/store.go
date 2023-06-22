package eventstore

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/query"
)

// New returns a thread-safe in-memory event store. The provided events are
// immediately inserted into the store.
//
// This event store is not production ready. It is intended to be used for
// testing and prototyping. In production, use the MongoDB event store instead.
// TODO(bounoable): List other event store implementations when they are ready.
func New(events ...event.Event) event.Store {
	store := &memstore{
		idMap:  make(map[uuid.UUID]event.Event),
		events: events,
	}
	for _, evt := range events {
		store.idMap[evt.ID()] = evt
	}
	return store
}

var (
	errEventNotFound  = errors.New("event not found")
	errDuplicateEvent = errors.New("duplicate event")
)

type memstore struct {
	mux    sync.RWMutex
	events []event.Event
	idMap  map[uuid.UUID]event.Event
}

// Insert inserts the provided events into the in-memory event store. If an
// event with the same ID already exists, an error is returned.
func (s *memstore) Insert(ctx context.Context, events ...event.Event) error {
	for _, evt := range events {
		if err := s.insert(ctx, evt); err != nil {
			return fmt.Errorf("%s:%s %w", evt.Name(), evt.ID(), err)
		}
	}
	return nil
}

func (s *memstore) insert(ctx context.Context, evt event.Event) error {
	if _, err := s.Find(ctx, evt.ID()); err == nil {
		return errDuplicateEvent
	}
	defer s.reslice()
	s.mux.Lock()
	defer s.mux.Unlock()
	s.idMap[evt.ID()] = evt
	return nil
}

// Find returns the event with the given UUID or errEventNotFound if no such
// event exists in the store.
func (s *memstore) Find(ctx context.Context, id uuid.UUID) (event.Event, error) {
	s.mux.RLock()
	defer s.mux.RUnlock()
	if evt := s.idMap[id]; evt != nil {
		return evt, nil
	}
	return nil, errEventNotFound
}

// Query returns a channel of events and a channel of errors. The events channel
// will emit all events in the store that match the given query. The order of
// the emitted events is determined by the query's sorting options. If the
// context is cancelled, the function will return immediately.
func (s *memstore) Query(ctx context.Context, q event.Query) (<-chan event.Event, <-chan error, error) {
	s.mux.RLock()
	defer s.mux.RUnlock()
	var events []event.Event
	for _, evt := range s.events {
		if query.Test(q, evt) {
			events = append(events, evt)
		}
	}
	events = event.SortMulti(events, q.Sortings()...)

	out := make(chan event.Event)
	errs := make(chan error)

	go func() {
		defer close(errs)
		defer close(out)
		for _, evt := range events {
			select {
			case <-ctx.Done():
				return
			case out <- evt:
			}
		}
	}()

	return out, errs, nil
}

// Delete removes the specified events from the store. Events are provided as a
// slice of event.Event.
func (s *memstore) Delete(ctx context.Context, events ...event.Event) error {
	defer s.reslice()
	s.mux.Lock()
	defer s.mux.Unlock()
	for _, evt := range events {
		delete(s.idMap, evt.ID())
	}
	return nil
}

func (s *memstore) reslice() {
	s.mux.Lock()
	defer s.mux.Unlock()
	s.events = s.events[:0]
	for _, evt := range s.idMap {
		s.events = append(s.events, evt)
	}
}
