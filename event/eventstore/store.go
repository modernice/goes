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
// Like the database-backed event stores, the returned store enforces
// uniqueness of event ids and of aggregate versions: inserting an event for
// an aggregate version that already exists in the store fails with an error,
// which makes optimistic concurrency control testable against this store.
// Insert is atomic — a failed insert does not apply any event of the batch.
//
// This event store is not production ready. It is intended to be used for
// testing and prototyping. In production, use the MongoDB event store instead.
// TODO(bounoable): List other event store implementations when they are ready.
func New(events ...event.Event) event.Store {
	store := &memstore{
		idMap:    make(map[uuid.UUID]event.Event),
		versions: make(map[versionKey]struct{}),
		events:   events,
	}
	for _, evt := range events {
		store.idMap[evt.ID()] = evt
		if key, ok := versionKeyOf(evt); ok {
			store.versions[key] = struct{}{}
		}
	}
	return store
}

var (
	errEventNotFound   = errors.New("event not found")
	errDuplicateEvent  = errors.New("duplicate event")
	errVersionConflict = errors.New("aggregate version already exists")
)

// versionKey identifies the unique aggregate version an event occupies.
// Mirroring the unique index of the MongoDB event store, only events that
// fully reference an aggregate (name, id, and a version greater than 0)
// occupy a version.
type versionKey struct {
	name    string
	id      uuid.UUID
	version int
}

func versionKeyOf(evt event.Event) (versionKey, bool) {
	id, name, v := evt.Aggregate()
	if name == "" || id == uuid.Nil || v <= 0 {
		return versionKey{}, false
	}
	return versionKey{name: name, id: id, version: v}, true
}

type memstore struct {
	mux      sync.RWMutex
	events   []event.Event
	idMap    map[uuid.UUID]event.Event
	versions map[versionKey]struct{}
}

// Insert inserts the provided events into the in-memory event store. If an
// event with the same id already exists, or an event occupies an aggregate
// version that already exists, an error is returned and no event of the
// batch is inserted.
func (s *memstore) Insert(ctx context.Context, events ...event.Event) error {
	s.mux.Lock()
	defer s.mux.Unlock()

	// Validate the whole batch before applying any event, mirroring the
	// unique constraints that database-backed stores enforce.
	ids := make(map[uuid.UUID]struct{}, len(events))
	versions := make(map[versionKey]struct{}, len(events))
	for _, evt := range events {
		if _, ok := s.idMap[evt.ID()]; ok {
			return fmt.Errorf("%s:%s %w", evt.Name(), evt.ID(), errDuplicateEvent)
		}
		if _, ok := ids[evt.ID()]; ok {
			return fmt.Errorf("%s:%s %w", evt.Name(), evt.ID(), errDuplicateEvent)
		}
		ids[evt.ID()] = struct{}{}

		key, ok := versionKeyOf(evt)
		if !ok {
			continue
		}
		if _, ok := s.versions[key]; ok {
			return versionConflictError(evt, key)
		}
		if _, ok := versions[key]; ok {
			return versionConflictError(evt, key)
		}
		versions[key] = struct{}{}
	}

	for _, evt := range events {
		s.idMap[evt.ID()] = evt
		s.events = append(s.events, evt)
		if key, ok := versionKeyOf(evt); ok {
			s.versions[key] = struct{}{}
		}
	}

	return nil
}

func versionConflictError(evt event.Event, key versionKey) error {
	return fmt.Errorf("%s:%s v%d of %q aggregate %s: %w", evt.Name(), evt.ID(), key.version, key.name, key.id, errVersionConflict)
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
	s.mux.Lock()
	defer s.mux.Unlock()

	for _, evt := range events {
		stored, ok := s.idMap[evt.ID()]
		if !ok {
			continue
		}
		delete(s.idMap, evt.ID())
		if key, ok := versionKeyOf(stored); ok {
			delete(s.versions, key)
		}
	}

	remaining := s.events[:0]
	for _, evt := range s.events {
		if _, ok := s.idMap[evt.ID()]; ok {
			remaining = append(remaining, evt)
		}
	}
	s.events = remaining

	return nil
}
