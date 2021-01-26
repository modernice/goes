// Package memstore provides an in-memory event.Store.
package memstore

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/query"
	"github.com/modernice/goes/event/stream"
)

type store struct {
	mux    sync.RWMutex
	events []event.Event
	idMap  map[uuid.UUID]event.Event
}

// New returns a new Store with events stored in it.
func New(events ...event.Event) event.Store {
	return &store{
		idMap:  make(map[uuid.UUID]event.Event),
		events: events,
	}
}

func (s *store) Insert(ctx context.Context, events ...event.Event) error {
	for _, evt := range events {
		if err := s.insert(ctx, evt); err != nil {
			return fmt.Errorf("%s:%s %w", evt.Name(), evt.ID(), err)
		}
	}
	return nil
}

func (s *store) insert(ctx context.Context, evt event.Event) error {
	if _, err := s.Find(ctx, evt.ID()); err == nil {
		return errors.New("")
	}
	defer s.reslice()
	s.mux.Lock()
	defer s.mux.Unlock()
	s.idMap[evt.ID()] = evt
	return nil
}

func (s *store) Find(ctx context.Context, id uuid.UUID) (event.Event, error) {
	s.mux.RLock()
	defer s.mux.RUnlock()
	if evt := s.idMap[id]; evt != nil {
		return evt, nil
	}
	return nil, errors.New("")
}

func (s *store) Query(ctx context.Context, q event.Query) (event.Stream, error) {
	s.mux.RLock()
	defer s.mux.RUnlock()
	var events []event.Event
	for _, evt := range s.events {
		if query.Test(q, evt) {
			events = append(events, evt)
		}
	}
	sorting := q.Sorting()
	events = event.Sort(events, sorting.Sort, sorting.Dir)
	return stream.InMemory(events...), nil
}

func (s *store) Delete(ctx context.Context, evt event.Event) error {
	defer s.reslice()
	s.mux.Lock()
	defer s.mux.Unlock()
	delete(s.idMap, evt.ID())
	return nil
}

func (s *store) reslice() {
	s.mux.Lock()
	defer s.mux.Unlock()
	events := s.events[:0]
	for _, evt := range s.idMap {
		events = append(events, evt)
	}
	s.events = events
}
