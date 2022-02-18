package eventstore

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/modernice/goes"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/query"
)

var (
	errEventNotFound  = errors.New("event not found")
	errDuplicateEvent = errors.New("duplicate event")
)

type memstore[ID goes.ID] struct {
	mux    sync.RWMutex
	events []event.Of[any, ID]
	idMap  map[ID]event.Of[any, ID]
}

// New returns a thread-safe in-memory event store.
func New[ID goes.ID](events ...event.Of[any, ID]) event.Store[ID] {
	return &memstore[ID]{
		idMap:  make(map[ID]event.Of[any, ID]),
		events: events,
	}
}

func (s *memstore[ID]) Insert(ctx context.Context, events ...event.Of[any, ID]) error {
	for _, evt := range events {
		if err := s.insert(ctx, evt); err != nil {
			return fmt.Errorf("%s:%s %w", evt.Name(), evt.ID(), err)
		}
	}
	return nil
}

func (s *memstore[ID]) insert(ctx context.Context, evt event.Of[any, ID]) error {
	if _, err := s.Find(ctx, evt.ID()); err == nil {
		return errDuplicateEvent
	}
	defer s.reslice()
	s.mux.Lock()
	defer s.mux.Unlock()
	s.idMap[evt.ID()] = evt
	return nil
}

func (s *memstore[ID]) Find(ctx context.Context, id ID) (event.Of[any, ID], error) {
	s.mux.RLock()
	defer s.mux.RUnlock()
	if evt := s.idMap[id]; evt != nil {
		return evt, nil
	}
	return nil, errEventNotFound
}

func (s *memstore[ID]) Query(ctx context.Context, q event.QueryOf[ID]) (<-chan event.Of[any, ID], <-chan error, error) {
	s.mux.RLock()
	defer s.mux.RUnlock()
	var events []event.Of[any, ID]
	for _, evt := range s.events {
		if query.Test(q, evt) {
			events = append(events, evt)
		}
	}
	events = event.SortMulti(events, q.Sortings()...)

	out := make(chan event.Of[any, ID])
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

func (s *memstore[ID]) Delete(ctx context.Context, events ...event.Of[any, ID]) error {
	defer s.reslice()
	s.mux.Lock()
	defer s.mux.Unlock()
	for _, evt := range events {
		delete(s.idMap, evt.ID())
	}
	return nil
}

func (s *memstore[ID]) reslice() {
	s.mux.Lock()
	defer s.mux.Unlock()
	s.events = s.events[:0]
	for _, evt := range s.idMap {
		s.events = append(s.events, evt)
	}
}
