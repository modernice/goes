package stream

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/consistency"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/internal/xerror"
)

// Option is an option for FromEvents.
type Option func(*stream)

type stream struct {
	isSorted            bool
	isGrouped           bool
	validateConsistency bool
	filters             []func(event.Event) bool

	stream       <-chan event.Event
	streamErrors []<-chan error
	inErrors     <-chan error

	acceptCtx  context.Context
	stopAccept context.CancelFunc
	acceptDone chan struct{}

	events   chan event.Event
	complete chan job

	groupReqs chan groupRequest

	// results   chan result
	out       chan aggregate.Applier
	outErrors chan error

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

type applier struct {
	job

	apply func(aggregate.Aggregate)
}

func Errors(errs ...<-chan error) Option {
	return func(s *stream) {
		s.streamErrors = append(s.streamErrors, errs...)
	}
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

func New(events <-chan event.Event, opts ...Option) (<-chan aggregate.Applier, <-chan error) {
	if events == nil {
		evts := make(chan event.Event)
		close(evts)
		events = evts
	}

	aes := stream{
		validateConsistency: true,
		stream:              events,
		acceptDone:          make(chan struct{}),
		events:              make(chan event.Event),
		complete:            make(chan job),
		groupReqs:           make(chan groupRequest),
		out:                 make(chan aggregate.Applier),
		outErrors:           make(chan error),
		closed:              make(chan struct{}),
	}
	for _, opt := range opts {
		opt(&aes)
	}
	aes.inErrors, _ = xerror.FanIn(aes.streamErrors...)

	aes.acceptCtx, aes.stopAccept = context.WithCancel(context.Background())
	go aes.acceptEvents()
	go aes.groupEvents()
	go aes.sortEvents()

	return aes.out, aes.outErrors
}

func Drain(ctx context.Context, str <-chan aggregate.Applier, errs ...<-chan error) ([]aggregate.Applier, error) {
	errChan, stop := xerror.FanIn(errs...)
	defer stop()

	out := make([]aggregate.Applier, 0, len(str))
	for {
		select {
		case <-ctx.Done():
			return out, ctx.Err()
		case err, ok := <-errChan:
			if !ok {
				errChan = nil
				break
			}
			return out, err
		case res, ok := <-str:
			if !ok {
				return out, nil
			}
			out = append(out, res)
		}
	}
}

// func (s *stream) Next(ctx context.Context) bool {
// 	// first check if the stream has been closed to ensure ErrClosed
// 	select {
// 	case <-s.closed:
// 		s.forceError(ErrClosed)
// 		return false
// 	default:
// 	}

// 	select {
// 	case <-ctx.Done():
// 		s.error(ctx.Err())
// 		return false
// 	case <-s.closed:
// 		s.forceError(ErrClosed)
// 		return false
// 	case r, ok := <-s.results:
// 		if !ok {
// 			return false
// 		}
// 		s.current = r
// 		return true
// 	}
// }

// func (s *stream) Current() (string, uuid.UUID) {
// 	return s.current.name, s.current.id
// }

// func (s *stream) Apply(a aggregate.Aggregate) error {
// 	if s.validateConsistency {
// 		if err := consistency.Validate(a, s.current.events...); err != nil {
// 			return err
// 		}
// 	}
// 	for _, evt := range s.current.events {
// 		a.ApplyEvent(evt)
// 	}
// 	a.TrackChange(s.current.events...)
// 	a.FlushChanges()
// 	return nil
// }

// func (s *stream) Err() error {
// 	s.errMux.RLock()
// 	defer s.errMux.RUnlock()
// 	return s.err
// }

func (s *stream) acceptEvents() {
	defer close(s.acceptDone)
	defer close(s.events)

	pending := make(map[job]bool)

	var prev job
L:
	for {
		select {
		case <-s.acceptCtx.Done():
			s.outErrors <- s.acceptCtx.Err()
		case err, ok := <-s.inErrors:
			if !ok {
				s.streamErrors = nil
				break
			}
			s.outErrors <- fmt.Errorf("event stream: %w", err)
			close(s.complete)
			return
		case evt, ok := <-s.stream:
			if !ok {
				break L
			}

			if s.shouldDiscard(evt) {
				break
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
	defer close(s.out)
	defer close(s.outErrors)
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

		if s.validateConsistency {
			a := aggregate.New(j.name, j.id)
			if err := consistency.Validate(a, events...); err != nil {
				s.outErrors <- err
				continue
			}
		}

		s.out <- applier{
			job: j,
			apply: func(a aggregate.Aggregate) {
				aggregate.ApplyHistory(a, events...)
			},
		}
	}
}

func (a applier) AggregateName() string {
	return a.name
}

func (a applier) AggregateID() uuid.UUID {
	return a.id
}

func (a applier) Apply(ag aggregate.Aggregate) {
	a.apply(ag)
}
