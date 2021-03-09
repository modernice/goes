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
)

var (
	// ErrNotFound is returned when an Event cannot be found in the Store.
	ErrNotFound = errors.New("event not found")

	// ErrDuplicate is returned when trying to insert an Event that already
	// exists in the Store.
	ErrDuplicate = errors.New("duplicate event")
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
		return ErrDuplicate
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
	return nil, ErrNotFound
}

func (s *store) Query(ctx context.Context, q event.Query) (<-chan event.Event, <-chan error, error) {
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
			out <- evt
		}
	}()

	return out, errs, nil
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
	s.events = s.events[:0]
	for _, evt := range s.idMap {
		s.events = append(s.events, evt)
	}
}
