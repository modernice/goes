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
)

var (
	// ErrClosed is returned by a Stream when trying to read from it or close it
	// after it has been closed.
	ErrClosed = errors.New("stream closed")
)

// Option is an option for FromEvents.
type Option func(*stream)

type stream struct {
	isSorted            bool
	isGrouped           bool
	validateConsistency bool
	filters             []func(event.Event) bool

	events event.Stream

	acceptCtx  context.Context
	stopAccept context.CancelFunc
	acceptDone chan struct{}

	queuesMux    sync.RWMutex
	queues       map[string]map[uuid.UUID]chan event.Event
	closedQueues map[chan event.Event]bool

	startedDrainsMux sync.RWMutex
	startedDrains    map[string]map[uuid.UUID]bool
	drainQueue       chan result

	results chan result
	current result

	errMux sync.RWMutex
	err    error

	closed chan struct{}
}

type result struct {
	name   string
	id     uuid.UUID
	events []event.Event
}

// Sorted returns an Option that optimizes Aggregate builds by giving the
// Stream information about the order of incoming Events from the event.Stream.
//
// When Sorted is disabled (which it is by default), the Stream sorts the
// collected Events for a specific Aggregate by the AggregateVersion of the
// Events before applying them to the Aggregate.
//
// Enable this option only if the underlying event.Stream guarantees that
// incoming Events are sorted by AggregateVersion.
func Sorted(v bool) Option {
	return func(s *stream) {
		s.isSorted = v
	}
}

// Grouped returns an Option that optimizes Aggregate builds by giving the
// Stream information about the order of incoming Events from the event.Stream.
//
// When Grouped is disabled, the Stream has to wait for the event.Stream to be
// drained before it can be sure no more Events will arrive for a specific
// Aggregate. When Grouped is enabled, the Stream knows when all Events for an
// Aggregate have been received and can therefore return the Aggregate as soon
// as its last Event has been received and applied.
//
// Grouped is disabled by default and should only be enabled if the correct
// order of events is guaranteed by the underlying event.Stream. Events are
// correctly ordered only if they're sequentially grouped by aggregate. Sorting
// within a group of Events does not matter if IsSorted is disabled (which it is
// by default). When IsSorted is enabled, Events within a group must be ordered
// by AggregateVersion.
//
// An example for correctly ordered events (with IsSorted disabled):
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
func Grouped(v bool) Option {
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

// Filter returns an Option that filters incoming Events before they're handled
// by the Stream. Events are passed to every fn in fns until a fn returns false.
// If any of fns returns false, the Event is discarded by the Stream.
func Filter(fns ...func(event.Event) bool) Option {
	return func(s *stream) {
		s.filters = append(s.filters, fns...)
	}
}

// New returns a Stream from an event.Stream. The returned Stream pulls Events
// from es by calling es.Next until es.Next returns false or s.Err returns a
// non-nil error. When s.Err returns a non-nil error, that error is also returned
// from as.Err.
//
// When the returned Stream is closed, the underlying event.Stream es is also
// closed.
func New(es event.Stream, opts ...Option) (as aggregate.Stream) {
	aes := stream{
		validateConsistency: true,
		events:              es,
		acceptDone:          make(chan struct{}),
		results:             make(chan result),
		queues:              make(map[string]map[uuid.UUID]chan event.Event),
		closedQueues:        make(map[chan event.Event]bool),
		drainQueue:          make(chan result),
		startedDrains:       make(map[string]map[uuid.UUID]bool),
		closed:              make(chan struct{}),
	}
	for _, opt := range opts {
		opt(&aes)
	}
	aes.acceptCtx, aes.stopAccept = context.WithCancel(context.Background())
	go aes.acceptEvents()
	go aes.drainAggregates()
	return &aes
}

func (s *stream) Next(ctx context.Context) bool {
	// first check if the stream has been closed to ensure ErrClosed
	select {
	case <-ctx.Done():
		s.error(ctx.Err())
		return false
	case <-s.closed:
		s.forceError(ErrClosed)
		return false
	default:
	}

	select {
	case <-ctx.Done():
		s.error(ctx.Err())
		return false
	case <-s.closed:
		s.forceError(ErrClosed)
		return false
	case r, ok := <-s.results:
		if !ok {
			return false
		}
		s.current = r
		return true
	}
}

func (s *stream) Current() (string, uuid.UUID) {
	return s.current.name, s.current.id
}

func (s *stream) Apply(a aggregate.Aggregate) error {
	if s.validateConsistency {
		if err := consistency.Validate(a, s.current.events...); err != nil {
			return err
		}
	}
	aggregate.ApplyHistory(a, s.current.events...)
	return nil
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

	// stop accepting events
	s.stopAccept()

	// wait until event stream is not used anymore
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.acceptDone:
	}

	// then close the event stream
	if err := s.events.Close(ctx); err != nil {
		return fmt.Errorf("close event stream: %w", err)
	}

	close(s.closed)

	return nil
}

func (s *stream) acceptEvents() {
	defer close(s.acceptDone)
	defer s.closeQueues()

	var prev event.Event
	for s.events.Next(s.acceptCtx) {
		evt := s.events.Event()

		if s.shouldDiscard(evt) {
			continue
		}

		name, id := evt.AggregateName(), evt.AggregateID()

		// start building the aggregate if it's the first event of an aggregate
		if !s.drainStarted(name, id) {
			if err := s.startDrain(name, id); err != nil {
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

func (s *stream) shouldDiscard(evt event.Event) bool {
	for _, fn := range s.filters {
		if !fn(evt) {
			return true
		}
	}
	return false
}

func (s *stream) drainStarted(name string, id uuid.UUID) bool {
	s.startedDrainsMux.RLock()
	defer s.startedDrainsMux.RUnlock()
	started, ok := s.startedDrains[name]
	if !ok {
		return false
	}
	return started[id]
}

func (s *stream) startDrain(name string, id uuid.UUID) error {
	s.startedDrainsMux.Lock()
	defer s.startedDrainsMux.Unlock()
	started, ok := s.startedDrains[name]
	if !ok {
		started = make(map[uuid.UUID]bool)
		s.startedDrains[name] = started
	}
	started[id] = true
	select {
	case <-s.closed:
	case s.drainQueue <- result{name: name, id: id}:
	}
	return nil
}

func (s *stream) closeQueues() {
	close(s.drainQueue)
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

func (s *stream) drainAggregates() {
	defer close(s.results)
	var wg sync.WaitGroup
	for r := range s.drainQueue {
		wg.Add(1)
		go s.drainAggregate(&wg, r.name, r.id)
	}
	wg.Wait()
}

func (s *stream) drainAggregate(wg *sync.WaitGroup, name string, id uuid.UUID) {
	defer wg.Done()
	select {
	case <-s.closed:
	case s.results <- result{
		name:   name,
		id:     id,
		events: s.drainEvents(name, id),
	}:
	}
}

func (s *stream) drainEvents(name string, id uuid.UUID) []event.Event {
	q := s.queue(name, id)

	var events []event.Event
	for evt := range q {
		events = append(events, evt)
	}

	if !s.isSorted {
		events = event.Sort(events, event.SortAggregateVersion, event.SortAsc)
	}

	return events
}

// error sets s.err to err if s.err == nil
func (s *stream) error(err error) {
	s.errMux.Lock()
	defer s.errMux.Unlock()
	select {
	case <-s.closed:
		return
	default:
	}
	if s.err == nil {
		s.err = err
	}
}

func (s *stream) forceError(err error) {
	s.errMux.Lock()
	defer s.errMux.Unlock()
	s.err = err
}
