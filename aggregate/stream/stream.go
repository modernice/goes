package stream

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/consistency"
	"github.com/modernice/goes/event"
	estream "github.com/modernice/goes/event/stream"
)

// Option is an option for FromEvents.
type Option func(*stream)

type stream struct {
	isSorted            bool
	validateConsistency bool

	events       event.Stream
	factoryFuncs map[string]func(uuid.UUID) aggregate.Aggregate

	queuesMux sync.RWMutex
	queues    map[string]map[uuid.UUID]chan event.Event

	startedBuildsMux sync.RWMutex
	startedBuilds    map[string]map[uuid.UUID]bool
	startQueue       chan aggregate.Aggregate

	results chan aggregate.Aggregate
	current aggregate.Aggregate

	errMux sync.RWMutex
	err    error
}

// AggregateFactory returns an Option that provides the factory function for
// aggregates with name as their name.
func AggregateFactory(name string, fn func(uuid.UUID) aggregate.Aggregate) Option {
	return func(s *stream) {
		s.factoryFuncs[name] = fn
	}
}

// IsSorted returns an Option that gives the Stream performance hints. When
// this option is enabled the Stream won't sort events by their aggregate
// version before applying them to the aggregate that's being built. The Stream
// will also return aggregates earlier after they're built because the Stream
// knows if an event is the last for an aggregate.
//
// This option is disabled by default and should only be enabled if a correct
// order of events is guaranteed. Events are correctly ordered only if they're
// sequentally grouped by aggregate and within the groups sorted by version.
//
// Here's an example for correctly sorted events (note how the groups don't need
// to be sorted, only the events within a group):
//
// 	name="foo" id="BBXXXXXX-XXXX-XXXX-XXXXXXXXXXXX" version=1
// 	name="foo" id="BBXXXXXX-XXXX-XXXX-XXXXXXXXXXXX" version=2
// 	name="foo" id="BBXXXXXX-XXXX-XXXX-XXXXXXXXXXXX" version=3
// 	name="foo" id="BBXXXXXX-XXXX-XXXX-XXXXXXXXXXXX" version=4
// 	name="bar" id="AXXXXXXX-XXXX-XXXX-XXXXXXXXXXXX" version=1
// 	name="bar" id="AXXXXXXX-XXXX-XXXX-XXXXXXXXXXXX" version=2
// 	name="bar" id="AXXXXXXX-XXXX-XXXX-XXXXXXXXXXXX" version=3
// 	name="bar" id="AXXXXXXX-XXXX-XXXX-XXXXXXXXXXXX" version=4
// 	name="foo" id="AAXXXXXX-XXXX-XXXX-XXXXXXXXXXXX" version=1
// 	name="foo" id="AAXXXXXX-XXXX-XXXX-XXXXXXXXXXXX" version=2
// 	name="foo" id="AAXXXXXX-XXXX-XXXX-XXXXXXXXXXXX" version=3
// 	name="foo" id="AAXXXXXX-XXXX-XXXX-XXXXXXXXXXXX" version=4
// 	name="bar" id="BXXXXXXX-XXXX-XXXX-XXXXXXXXXXXX" version=1
// 	name="bar" id="BXXXXXXX-XXXX-XXXX-XXXXXXXXXXXX" version=2
// 	name="bar" id="BXXXXXXX-XXXX-XXXX-XXXXXXXXXXXX" version=3
// 	name="bar" id="BXXXXXXX-XXXX-XXXX-XXXXXXXXXXXX" version=4
func IsSorted(v bool) Option {
	return func(s *stream) {
		s.isSorted = v
	}
}

// ValidateConsistency returns an Option that controls if the consistency of
// events gets validates before building an aggregate. This option is enabled by
// default and should only be disabled if the consistency of events is
// guaranteed beforehand or if it's explicitly desired to put an aggregate into
// an invalid state.
func ValidateConsistency(v bool) Option {
	return func(s *stream) {
		s.validateConsistency = v
	}
}

// FromEvents returns a Stream from an event.Stream. The returned Stream pulls
// events from es by calling s.Next until s.Next returns false or s.Err
// returns a non-nil error. When s.Err returns a non-nil error, that error is
// also returned from as.Err.
func FromEvents(es event.Stream, opts ...Option) (as aggregate.Stream) {
	aes := stream{
		validateConsistency: true,
		events:              es,
		factoryFuncs:        make(map[string]func(uuid.UUID) aggregate.Aggregate),
		results:             make(chan aggregate.Aggregate),
		queues:              make(map[string]map[uuid.UUID]chan event.Event),
		startQueue:          make(chan aggregate.Aggregate),
		startedBuilds:       make(map[string]map[uuid.UUID]bool),
	}
	for _, opt := range opts {
		opt(&aes)
	}

	go aes.acceptEvents()
	go aes.buildAggregates()
	return &aes
}

func (s *stream) Next(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		s.error(ctx.Err())
		return false
	case a, ok := <-s.results:
		if !ok {
			return false
		}
		s.current = a
		return true
	}
}

func (s *stream) Aggregate() aggregate.Aggregate {
	return s.current
}

func (s *stream) Err() error {
	s.errMux.RLock()
	defer s.errMux.RUnlock()
	return s.err
}

func (s *stream) Close(ctx context.Context) error {
	return s.events.Close(ctx)
}

func (s *stream) acceptEvents() {
	defer s.closeQueues()

	ctx := context.Background()
	for s.events.Next(ctx) {
		evt := s.events.Event()
		name, id := evt.AggregateName(), evt.AggregateID()

		// start building the aggregate if it's the first event of an aggregate
		if !s.buildStarted(name, id) {
			s.startBuild(name, id)
		}

		s.queueEvent(evt)
	}

	if err := s.events.Err(); err != nil {
		if errors.Is(err, estream.ErrClosed) {
			err = ErrClosed
		}
		s.error(err)
	}
}

func (s *stream) buildStarted(name string, id uuid.UUID) bool {
	s.startedBuildsMux.RLock()
	defer s.startedBuildsMux.RUnlock()
	started, ok := s.startedBuilds[name]
	if !ok {
		return false
	}
	return started[id]
}

func (s *stream) startBuild(name string, id uuid.UUID) {
	a := s.newAggregate(name, id)
	s.startedBuildsMux.Lock()
	defer s.startedBuildsMux.Unlock()
	started, ok := s.startedBuilds[name]
	if !ok {
		started = make(map[uuid.UUID]bool)
		s.startedBuilds[name] = started
	}
	started[id] = true
	s.startQueue <- a
}

func (s *stream) closeQueues() {
	close(s.startQueue)
	s.queuesMux.RLock()
	defer s.queuesMux.RUnlock()
	for _, queues := range s.queues {
		for _, q := range queues {
			close(q)
		}
	}
}

func (s *stream) queueEvent(evt event.Event) {
	s.queue(evt.AggregateName(), evt.AggregateID()) <- evt
}

func (s *stream) queue(name string, id uuid.UUID) chan event.Event {
	if q, ok := s.getQueue(name, id); ok {
		return q
	}
	return s.newQueue(name, id)
}

func (s *stream) getQueue(name string, id uuid.UUID) (chan event.Event, bool) {
	s.queuesMux.RLock()
	defer s.queuesMux.RUnlock()
	queues, ok := s.queues[name]
	if !ok {
		return nil, false
	}
	q, ok := queues[id]
	return q, ok
}

func (s *stream) newQueue(name string, id uuid.UUID) chan event.Event {
	s.queuesMux.Lock()
	defer s.queuesMux.Unlock()
	queues, ok := s.queues[name]
	if !ok {
		queues = make(map[uuid.UUID]chan event.Event)
		s.queues[name] = queues
	}
	q, ok := queues[id]
	if !ok {
		q = make(chan event.Event)
		queues[id] = q
	}
	return q
}

func (s *stream) buildAggregates() {
	defer close(s.results)
	var wg sync.WaitGroup
	for a := range s.startQueue {
		wg.Add(1)
		go s.buildAggregate(&wg, a)
	}
	wg.Wait()
}

func (s *stream) buildAggregate(wg *sync.WaitGroup, a aggregate.Aggregate) {
	defer wg.Done()
	if err := s.build(a); err != nil {
		s.error(err)
		return
	}
	s.results <- a
}

func (s *stream) build(a aggregate.Aggregate) error {
	q := s.queue(a.AggregateName(), a.AggregateID())

	var events []event.Event
	for evt := range q {
		events = append(events, evt)
	}

	if !s.isSorted {
		events = event.Sort(events, event.SortAggregateVersion, event.SortAsc)
	}

	if s.validateConsistency {
		if err := consistency.Validate(a, events...); err != nil {
			return fmt.Errorf("validate consistency: %w", err)
		}
	}

	for _, evt := range events {
		a.ApplyEvent(evt)
	}

	if err := a.TrackChange(events...); err != nil {
		return fmt.Errorf("track change: %w\n\n%#v", err, events)
	}

	return nil
}

func (s *stream) newAggregate(name string, id uuid.UUID) aggregate.Aggregate {
	fn := s.factoryFuncs[name]
	return fn(id)
}

// error sets s.err to err if s.err == nil
func (s *stream) error(err error) {
	s.errMux.Lock()
	defer s.errMux.Unlock()
	if s.err == nil {
		s.err = err
	}
}
