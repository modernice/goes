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

	stream event.Stream

	acceptCtx  context.Context
	stopAccept context.CancelFunc
	acceptDone chan struct{}

	events   chan event.Event
	complete chan job

	groupReqs chan groupRequest

	results chan result
	current result

	errMux sync.RWMutex
	err    error

	closed chan struct{}
}

type job struct {
	name string
	id   uuid.UUID
}

type groupRequest struct {
	job
	out chan []event.Event
}

type result struct {
	job
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
		stream:              es,
		acceptDone:          make(chan struct{}),
		events:              make(chan event.Event),
		complete:            make(chan job),
		groupReqs:           make(chan groupRequest),
		results:             make(chan result),
		closed:              make(chan struct{}),
	}
	for _, opt := range opts {
		opt(&aes)
	}
	aes.acceptCtx, aes.stopAccept = context.WithCancel(context.Background())
	go aes.acceptEvents()
	go aes.groupEvents()
	go aes.sortEvents()
	return &aes
}

func (s *stream) Next(ctx context.Context) bool {
	// first check if the stream has been closed to ensure ErrClosed
	select {
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
	for _, evt := range s.current.events {
		a.ApplyEvent(evt)
	}
	a.TrackChange(s.current.events...)
	a.FlushChanges()
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

	s.stopAccept()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.acceptDone:
	}

	if err := s.stream.Close(ctx); err != nil {
		return fmt.Errorf("close event stream: %w", err)
	}

	close(s.closed)

	return nil
}

func (s *stream) acceptEvents() {
	defer close(s.acceptDone)
	defer close(s.events)

	pending := make(map[job]bool)

	var prev job
	for s.stream.Next(s.acceptCtx) {
		evt := s.stream.Event()

		if s.shouldDiscard(evt) {
			continue
		}

		s.events <- evt

		j := job{
			name: evt.AggregateName(),
			id:   evt.AggregateID(),
		}
		pending[j] = true

		if s.isGrouped && prev.name != "" && prev != j {
			s.complete <- prev
			delete(pending, prev)
		}

		prev = j
	}

	if err := s.stream.Err(); err != nil {
		s.error(fmt.Errorf("event stream: %w", err))
		close(s.complete)
		return
	}

	go func() {
		defer close(s.complete)
		for j := range pending {
			s.complete <- j
		}
	}()
}

func (s *stream) shouldDiscard(evt event.Event) bool {
	for _, fn := range s.filters {
		if !fn(evt) {
			return true
		}
	}
	return false
}

func (s *stream) groupEvents() {
	groups := make(map[job][]event.Event)
	events := s.events
	groupReqs := s.groupReqs
	for {
		if events == nil && groupReqs == nil {
			return
		}

		select {
		case evt, ok := <-events:
			if !ok {
				events = nil
				break
			}
			j := job{evt.AggregateName(), evt.AggregateID()}
			groups[j] = append(groups[j], evt)
		case req, ok := <-groupReqs:
			if !ok {
				groupReqs = nil
				break
			}
			req.out <- groups[req.job]
			delete(groups, req.job)
		}
	}
}

func (s *stream) sortEvents() {
	defer close(s.results)
	defer close(s.groupReqs)

	for j := range s.complete {
		req := groupRequest{
			job: j,
			out: make(chan []event.Event),
		}
		s.groupReqs <- req
		events := <-req.out

		if !s.isSorted {
			events = event.Sort(events, event.SortAggregateVersion, event.SortAsc)
		}

		s.results <- result{
			job:    j,
			events: events,
		}
	}
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
