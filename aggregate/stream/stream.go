package stream

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/consistency"
	"github.com/modernice/goes/event"
)

// Option is an option for FromEvents.
type Option func(*stream)

type stream struct {
	isSorted            bool
	isGrouped           bool
	validateConsistency bool

	events       event.Stream
	factoryFuncs map[string]func(uuid.UUID) aggregate.Aggregate

	acceptCtx  context.Context
	stopAccept context.CancelFunc

	queuesMux    sync.RWMutex
	queues       map[string]map[uuid.UUID]chan event.Event
	closedQueues map[chan event.Event]bool

	startedBuildsMux sync.RWMutex
	startedBuilds    map[string]map[uuid.UUID]bool
	startQueue       chan aggregate.Aggregate

	results chan aggregate.Aggregate
	current aggregate.Aggregate

	errMux sync.RWMutex
	err    error

	closed chan struct{}
}

// AggregateFactory returns an Option that provides the factory function for
// aggregates with name as their name.
func AggregateFactory(name string, fn func(uuid.UUID) aggregate.Aggregate) Option {
	return func(s *stream) {
		s.factoryFuncs[name] = fn
	}
}

// IsSorted returns an Option that optimizes Aggregate builds by giving the
// Stream information about the order of incoming Events from the event.Stream.
//
// When IsSorted is enabled (which it is by default), the Stream sorts the
// collected Events for a specific Aggregate by the AggregateVersion of the
// Events before applying them to the Aggregate.
//
// Disable this option only if the underlying event.Stream guarantees that
// incoming Events are sorted by AggregateVersion.
func IsSorted(v bool) Option {
	return func(s *stream) {
		s.isSorted = v
	}
}

// IsGrouped returns an Option that optimizes Aggregate builds by giving the
// Stream information about the order of incoming Events from the event.Stream.
//
// When IsGrouped is disabled, the Stream has to wait for the event.Stream to be
// drained before it can be sure no more Events will arrive for a specific
// Aggregate. When IsGrouped is enabled, the Stream knows when all Events for an
// Aggregate have been received and can therefore return the Aggregate as soon
// as its last Event has been received and applied.
//
// IsGrouped is disabled by default and should only be enabled if the correct
// order of events is guaranteed by the event.Stream. Events are correctly
// ordered only if they're sequentally grouped by aggregate. Sorting within a
// group of Events does not matter if IsSorted is disabled (which it is by
// default). When IsSorted is enabled, Events within a group must be ordered by
// AggregateVersion.
//
// What's not important is the order of the Event groups; only that Events for
// a an instance of an Aggregate come in sequentially.
//
// An example for correctly ordered events (when IsSorted is disabled):
//
// 	name="foo" id="BBXXXXXX-XXXX-XXXX-XXXXXXXXXXXX" version=2
// 	name="foo" id="BBXXXXXX-XXXX-XXXX-XXXXXXXXXXXX" version=1
// 	name="foo" id="BBXXXXXX-XXXX-XXXX-XXXXXXXXXXXX" version=4
// 	name="foo" id="BBXXXXXX-XXXX-XXXX-XXXXXXXXXXXX" version=3
// 	name="bar" id="AXXXXXXX-XXXX-XXXX-XXXXXXXXXXXX" version=1
// 	name="bar" id="AXXXXXXX-XXXX-XXXX-XXXXXXXXXXXX" version=2
// 	name="bar" id="AXXXXXXX-XXXX-XXXX-XXXXXXXXXXXX" version=3
// 	name="bar" id="AXXXXXXX-XXXX-XXXX-XXXXXXXXXXXX" version=4
// 	name="foo" id="AAXXXXXX-XXXX-XXXX-XXXXXXXXXXXX" version=4
// 	name="foo" id="AAXXXXXX-XXXX-XXXX-XXXXXXXXXXXX" version=3
// 	name="foo" id="AAXXXXXX-XXXX-XXXX-XXXXXXXXXXXX" version=2
// 	name="foo" id="AAXXXXXX-XXXX-XXXX-XXXXXXXXXXXX" version=1
// 	name="bar" id="BXXXXXXX-XXXX-XXXX-XXXXXXXXXXXX" version=2
// 	name="bar" id="BXXXXXXX-XXXX-XXXX-XXXXXXXXXXXX" version=1
// 	name="bar" id="BXXXXXXX-XXXX-XXXX-XXXXXXXXXXXX" version=3
// 	name="bar" id="BXXXXXXX-XXXX-XXXX-XXXXXXXXXXXX" version=4
func IsGrouped(v bool) Option {
	return func(s *stream) {
		s.isGrouped = v
	}
}

// ValidateConsistency returns an Option that optimizes Aggregate builds by
// controlling if the consistency of Events is validated before building an
// Aggregate from those Events.
//
// This option is enabled by default and should only be disabled if the
// consistency of Events is guaranteed by the underlying event.Stream or if it's
// explicitly desired to put an Aggregate into an invalid state.
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
		closedQueues:        make(map[chan event.Event]bool),
		startQueue:          make(chan aggregate.Aggregate),
		startedBuilds:       make(map[string]map[uuid.UUID]bool),
		closed:              make(chan struct{}),
	}
	for _, opt := range opts {
		opt(&aes)
	}
	aes.acceptCtx, aes.stopAccept = context.WithCancel(context.Background())
	go aes.acceptEvents()
	go aes.buildAggregates()
	return &aes
}

func (s *stream) Next(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		s.error(ctx.Err())
		return false
	case <-s.closed:
		s.error(ErrClosed)
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
	select {
	case <-ctx.Done():
		return ctx.Err()
	case _, ok := <-s.closed:
		if !ok {
			return ErrClosed
		}
	default:
	}
	close(s.closed)
	s.stopAccept()
	return nil
}

func (s *stream) acceptEvents() {
	defer s.closeQueues()

	var prev event.Event
	for s.events.Next(s.acceptCtx) {
		evt := s.events.Event()
		name, id := evt.AggregateName(), evt.AggregateID()

		// start building the aggregate if it's the first event of an aggregate
		if !s.buildStarted(name, id) {
			if err := s.startBuild(name, id); err != nil {
				s.error(fmt.Errorf("start build %s(%s): %w", name, id, err))
				continue
			}
		}
		s.queueEvent(evt)

		// if the event stream is grouped, check if prev belongs to another
		// aggregate: if so, close the previous aggregates event queue
		if s.isGrouped && prev != nil &&
			(prev.AggregateName() != evt.AggregateName() ||
				prev.AggregateID() != evt.AggregateID()) {
			s.closeQueue(s.queue(prev.AggregateName(), prev.AggregateID()))
		}

		prev = evt
	}

	if err := s.events.Err(); err != nil {
		s.error(fmt.Errorf("event stream: %w", err))
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

func (s *stream) startBuild(name string, id uuid.UUID) error {
	a, err := s.newAggregate(name, id)
	if err != nil {
		return fmt.Errorf("new %q aggregate: %w", name, err)
	}

	s.startedBuildsMux.Lock()
	defer s.startedBuildsMux.Unlock()
	started, ok := s.startedBuilds[name]
	if !ok {
		started = make(map[uuid.UUID]bool)
		s.startedBuilds[name] = started
	}
	started[id] = true
	s.startQueue <- a
	return nil
}

func (s *stream) closeQueues() {
	close(s.startQueue)
	s.queuesMux.RLock()
	defer s.queuesMux.RUnlock()
	for _, queues := range s.queues {
		for _, q := range queues {
			s.queuesMux.RUnlock()
			s.closeQueue(q)
			s.queuesMux.RLock()
		}
	}
}

func (s *stream) closeQueue(q chan event.Event) {
	if !s.queueClosed(q) {
		s.queuesMux.Lock()
		defer s.queuesMux.Unlock()
		s.closedQueues[q] = true
		close(q)
	}
}

func (s *stream) queueClosed(q chan event.Event) bool {
	s.queuesMux.RLock()
	defer s.queuesMux.RUnlock()
	return s.closedQueues[q]
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

	a.TrackChange(events...)

	return nil
}

func (s *stream) newAggregate(name string, id uuid.UUID) (aggregate.Aggregate, error) {
	fn, ok := s.factoryFuncs[name]
	if !ok {
		return nil, ErrNoFactory
	}
	return fn(id), nil
}

// error sets s.err to err if s.err == nil
func (s *stream) error(err error) {
	s.errMux.Lock()
	defer s.errMux.Unlock()
	if s.err == nil {
		s.err = err
	}
}
