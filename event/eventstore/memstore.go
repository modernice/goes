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

var (
	errEventNotFound  = errors.New("event not found")
	errDuplicateEvent = errors.New("duplicate event")
)

type memstore[D any] struct {
	mux    sync.RWMutex
	events []event.Event[D]
	idMap  map[uuid.UUID]event.Event[D]
}

// New returns a thread-safe in-memory event store.
func New[D any](events ...event.Event[D]) event.Store[D] {
	return &memstore[D]{
		idMap:  make(map[uuid.UUID]event.Event[D]),
		events: events,
	}
}

func (s *memstore[D]) Insert(ctx context.Context, events ...event.Event[D]) error {
	for _, evt := range events {
		if err := s.insert(ctx, evt); err != nil {
			return fmt.Errorf("%s:%s %w", evt.Name(), evt.ID(), err)
		}
	}
	return nil
}

func (s *memstore[D]) insert(ctx context.Context, evt event.Event[D]) error {
	if _, err := s.Find(ctx, evt.ID()); err == nil {
		return errDuplicateEvent
	}
	defer s.reslice()
	s.mux.Lock()
	defer s.mux.Unlock()
	s.idMap[evt.ID()] = evt
	return nil
}

func (s *memstore[D]) Find(ctx context.Context, id uuid.UUID) (event.Event[D], error) {
	s.mux.RLock()
	defer s.mux.RUnlock()
	if evt := s.idMap[id]; evt != nil {
		return evt, nil
	}
	return nil, errEventNotFound
}

func (s *memstore[D]) Query(ctx context.Context, q event.Query) (<-chan event.Event[D], <-chan error, error) {
	s.mux.RLock()
	defer s.mux.RUnlock()
	var events []event.Event[D]
	for _, evt := range s.events {
		if query.Test(q, evt) {
			events = append(events, evt)
		}
	}
	events = event.SortMulti(events, q.Sortings()...)

	out := make(chan event.Event[D])
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

func (s *memstore[D]) Delete(ctx context.Context, events ...event.Event[D]) error {
	defer s.reslice()
	s.mux.Lock()
	defer s.mux.Unlock()
	for _, evt := range events {
		delete(s.idMap, evt.ID())
	}
	return nil
}

func (s *memstore[D]) reslice() {
	s.mux.Lock()
	defer s.mux.Unlock()
	s.events = s.events[:0]
	for _, evt := range s.idMap {
		s.events = append(s.events, evt)
	}
}
