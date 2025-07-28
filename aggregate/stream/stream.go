package stream

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/internal/softdelete"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/helper/streams"
)

// Option is a stream option.
type Option func(*options)

type options struct {
	isSorted            bool
	isGrouped           bool
	validateConsistency bool
	withSoftDeleted     bool
	filters             []func(event.Event) bool
	streamErrors        []<-chan error
}

type stream struct {
	options

	stream     <-chan event.Evt[any]
	inErrors   <-chan error
	stopErrors func()

	acceptDone chan struct{}

	events   chan event.Event
	complete chan job

	groupReqs chan groupRequest

	out       chan aggregate.History
	outErrors chan error
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

// Errors returns an Option that provides a Stream with error channels. A Stream
// will cancel its operation as soon as an error can be received from one of the
// error channels.
func Errors(errs ...<-chan error) Option {
	return func(opts *options) {
		opts.streamErrors = append(opts.streamErrors, errs...)
	}
}

// Sorted returns an Option that optimizes aggregate builds by informing the
// Stream about the sort order of incoming events.
//
// When Sorted is disabled (default), the Stream sorts the collected events
// for each aggregate by AggregateVersion before applying them.
//
// Enable this option only if the input event stream guarantees that events
// are already sorted by AggregateVersion for each aggregate.
func Sorted(v bool) Option {
	return func(opts *options) {
		opts.isSorted = v
	}
}

// Grouped returns an Option that optimizes aggregate builds by informing the
// Stream about the grouping of incoming events.
//
// When Grouped is disabled (default), the Stream must wait for the input
// stream to close before returning aggregates, as it cannot determine when
// all events for a specific aggregate have been received.
//
// When Grouped is enabled, the Stream assumes events are grouped by aggregate
// (name, ID) and can return each aggregate as soon as it receives the last
// event for that aggregate.
//
// Enable this option only if the input stream guarantees that events are
// sequentially grouped by aggregate. Events within each group can be in any
// order unless Sorted is also enabled.
//
// Example of correctly grouped events:
//
//	// Group 1: foo/BB...
//	name="foo" id="BB..." version=2
//	name="foo" id="BB..." version=1
//	name="foo" id="BB..." version=4
//	name="foo" id="BB..." version=3
//	// Group 2: bar/AA...
//	name="bar" id="AA..." version=1
//	name="bar" id="AA..." version=2
//	// Group 3: foo/AA... (different ID)
//	name="foo" id="AA..." version=1
//	name="foo" id="AA..." version=2
func Grouped(v bool) Option {
	return func(opts *options) {
		opts.isGrouped = v
	}
}

// ValidateConsistency returns an Option that controls whether event consistency
// is validated before building aggregates.
//
// When enabled (default), the Stream validates that events form a consistent
// sequence (no gaps in version numbers, correct aggregate references, etc.)
// before applying them to build an aggregate.
//
// Disable this option only if event consistency is guaranteed by the source
// or if you explicitly want to allow aggregates in potentially invalid states.
func ValidateConsistency(v bool) Option {
	return func(opts *options) {
		opts.validateConsistency = v
	}
}

// Filter returns an Option that filters incoming events before processing.
// Events are passed through each filter function in order. If any filter
// returns false, the event is discarded.
func Filter(fns ...func(event.Event) bool) Option {
	return func(opts *options) {
		opts.filters = append(opts.filters, fns...)
	}
}

// WithSoftDeleted returns an Option that controls whether soft-deleted
// aggregates are included in the output stream.
//
// By default (false), soft-deleted aggregates are excluded from results.
// Set to true to include soft-deleted aggregates in the output.
func WithSoftDeleted(v bool) Option {
	return func(opts *options) {
		opts.withSoftDeleted = v
	}
}

// New creates a stream that converts events into aggregate Histories.
// It takes a channel of events and returns both a channel of aggregate
// Histories and an error channel.
//
// Each History can be applied to an aggregate to build its current state.
//
// Example usage:
//
//	var events <-chan event.Event
//	histories, errs := stream.New(ctx, events)
//
//	// Process results
//	for history := range histories {
//		foo := newFoo(history.Aggregate().ID)
//		history.Apply(foo)
//		// foo now contains the current state
//	}
func New(ctx context.Context, events <-chan event.Event, opts ...Option) (<-chan aggregate.History, <-chan error) {
	return NewOf(ctx, events, opts...)
}

// NewOf creates a typed stream that converts events into aggregate Histories.
// It works the same as New but accepts a typed event channel.
//
// Each History can be applied to an aggregate to build its current state.
//
// Example usage:
//
//	var events <-chan event.Of[MyData]
//	histories, errs := stream.NewOf(ctx, events)
//
//	// Process results
//	for history := range histories {
//		foo := newFoo(history.Aggregate().ID)
//		history.Apply(foo)
//		// foo now contains the current state
//	}
func NewOf[D any, Event event.Of[D]](ctx context.Context, events <-chan Event, opts ...Option) (<-chan aggregate.History, <-chan error) {
	if events == nil {
		evts := make(chan Event)
		close(evts)
		events = evts
	}

	aes := stream{
		options:    options{validateConsistency: true},
		stream:     streams.Map(ctx, events, func(e Event) event.Evt[any] { return event.Any[D](e) }),
		acceptDone: make(chan struct{}),
		events:     make(chan event.Event),
		complete:   make(chan job),
		groupReqs:  make(chan groupRequest),
		out:        make(chan aggregate.History),
		outErrors:  make(chan error),
	}
	for _, opt := range opts {
		opt(&aes.options)
	}

	aes.inErrors, aes.stopErrors = streams.FanIn(aes.streamErrors...)

	go aes.acceptEvents()
	go aes.groupEvents()
	go aes.sortEvents()

	return aes.out, aes.outErrors
}

func (s *stream) acceptEvents() {
	defer close(s.complete)
	defer close(s.acceptDone)
	defer close(s.events)
	defer s.stopErrors()

	pending := make(map[job]bool)

	var prev job
L:
	for {
		select {
		case err, ok := <-s.inErrors:
			if !ok {
				s.inErrors = nil
				break
			}
			s.outErrors <- fmt.Errorf("event stream: %w", err)
			break L
		case evt, ok := <-s.stream:
			if !ok {
				break L
			}

			if s.shouldDiscard(evt) {
				break
			}

			s.events <- evt

			id, name, _ := evt.Aggregate()

			j := job{
				name: name,
				id:   id,
			}

			pending[j] = true

			if s.isGrouped && prev.name != "" && prev != j {
				s.complete <- prev
				delete(pending, prev)
			}

			prev = j
		}
	}

	for j := range pending {
		s.complete <- j
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
			id, name, _ := evt.Aggregate()
			j := job{name, id}
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
			if err := aggregate.ValidateConsistency(a.Ref(), a.AggregateVersion(), events); err != nil {
				s.outErrors <- err
				continue
			}
		}

		if !s.withSoftDeleted && softdelete.SoftDeleted(events) {
			continue
		}

		s.out <- applier{
			job:   j,
			apply: func(a aggregate.Aggregate) { aggregate.ApplyHistory(a, events) },
		}
	}
}

//jotbot:ignore
func (a applier) Aggregate() aggregate.Ref {
	return aggregate.Ref{Name: a.name, ID: a.id}
}

//jotbot:ignore
func (a applier) Apply(ag aggregate.Aggregate) {
	a.apply(ag)
}
