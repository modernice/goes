package stream

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/event"
)

var (
	// ErrClosed means there was a call to (*stream).Next after the stream was closed.
	ErrClosed = errors.New("stream closed")
)

// Option is a stream option.
type Option func(*stream)

type stream struct {
	ctx  context.Context
	stop context.CancelFunc

	factories map[string]func(uuid.UUID) aggregate.Aggregate

	events     event.Cursor
	eventErrs  chan error
	eventsDone chan struct{}

	results    chan result
	resultDone chan struct{}

	stacksMux sync.RWMutex
	stacks    map[string]map[uuid.UUID][]event.Event

	current aggregate.Aggregate
	err     error
}

type result struct {
	aggregate aggregate.Aggregate
	err       error
}

func New(ctx context.Context, cur event.Cursor, opts ...Option) aggregate.Cursor {
	ctx, cancel := context.WithCancel(ctx)
	s := &stream{
		ctx:        ctx,
		stop:       cancel,
		events:     cur,
		eventErrs:  make(chan error, 1),
		eventsDone: make(chan struct{}),
		factories:  make(map[string]func(uuid.UUID) aggregate.Aggregate),
		results:    make(chan result),
		resultDone: make(chan struct{}),
		stacks:     make(map[string]map[uuid.UUID][]event.Event),
	}
	for _, opt := range opts {
		opt(s)
	}

	go s.acceptEvents()
	go s.buildAggregates()

	return s
}

func AggregateFactory(name string, newFunc func(uuid.UUID) aggregate.Aggregate) Option {
	return func(s *stream) {
		s.factories[name] = newFunc
	}
}

func (s *stream) Next(ctx context.Context) bool {
	select {
	case <-s.ctx.Done():
		s.err = ErrClosed
		return false
	default:
	}

	select {
	case <-s.ctx.Done():
		s.err = ErrClosed
		return false
	case <-ctx.Done():
		s.current = nil
		s.err = ctx.Err()
		return false
	case err := <-s.eventErrs:
		s.err = err
		return s.err == nil
	case r, ok := <-s.results:
		if !ok {
			s.err = nil
			return false
		}
		s.err = r.err
		s.current = r.aggregate
		return s.err == nil
	}
}

func (s *stream) Aggregate() aggregate.Aggregate {
	return s.current
}

func (s *stream) Err() error {
	return s.err
}

func (s *stream) Close(ctx context.Context) error {
	s.stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.resultDone:
		return nil
	}
}

func (s *stream) acceptEvents() {
	defer close(s.eventsDone)
	for s.events.Next(s.ctx) {
		s.addEvent(s.events.Event())
	}
	if err := s.events.Err(); err != nil {
		s.eventErrs <- err
	}
}

func (s *stream) addEvent(evt event.Event) {
	name := evt.AggregateName()
	id := evt.AggregateID()
	st := s.stack(name, id)
	st = append(st, evt)

	s.stacksMux.Lock()
	defer s.stacksMux.Unlock()
	if s.stacks[name] == nil {
		s.stacks[name] = make(map[uuid.UUID][]event.Event)
	}
	s.stacks[name][id] = st
}

func (s *stream) stackID(name string, id uuid.UUID) string {
	return fmt.Sprintf("%s:%s", name, id)
}

func (s *stream) stack(name string, id uuid.UUID) []event.Event {
	s.stacksMux.RLock()
	defer s.stacksMux.RUnlock()
	stacks, ok := s.stacks[name]
	if !ok {
		stacks = make(map[uuid.UUID][]event.Event)
	}
	return stacks[id]
}

func (s *stream) buildAggregates() {
	defer close(s.resultDone)
	defer close(s.results)

	<-s.eventsDone

	s.stacksMux.RLock()
	defer s.stacksMux.RUnlock()

	var wg sync.WaitGroup
	wg.Add(len(s.stacks))

	for name, stacks := range s.stacks {
		name := name
		stacks := stacks
		go func() {
			defer wg.Done()
			s.buildAggregateName(name, stacks)
		}()
	}
	wg.Wait()
}

func (s *stream) buildAggregateName(name string, stacks map[uuid.UUID][]event.Event) {
	for _, stack := range stacks {
		a, err := s.newAggregate(name, stack...)
		s.results <- result{
			aggregate: a,
			err:       err,
		}
	}
}

func (s *stream) newAggregate(name string, events ...event.Event) (aggregate.Aggregate, error) {
	events = event.Sort(events, event.SortAggregateVersion, event.SortAsc)

	if len(events) == 0 {
		return nil, errors.New("cannot build aggregate without any events")
	}

	fn, ok := s.factories[name]
	if !ok {
		return nil, fmt.Errorf("no factory registered for %q aggregate", name)
	}

	a := fn(events[0].AggregateID())
	if err := aggregate.ApplyHistory(a, events...); err != nil {
		return a, fmt.Errorf("apply history: %w", err)
	}

	return a, nil
}
