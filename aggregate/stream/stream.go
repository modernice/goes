package stream

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/helper/streams"
)

// Option is a stream option.
type Option[D any] func(*stream[D])

type stream[D any] struct {
	isSorted            bool
	isGrouped           bool
	validateConsistency bool
	filters             []func(event.Of[D]) bool

	stream       <-chan event.Of[D]
	streamErrors []<-chan error
	inErrors     <-chan error
	stopErrors   func()

	acceptDone chan struct{}

	events   chan event.Of[D]
	complete chan job

	groupReqs chan groupRequest[D]

	out       chan aggregate.History
	outErrors chan error
}

type job struct {
	name string
	id   uuid.UUID
}

type groupRequest[D any] struct {
	job
	out chan []event.Of[D]
}

type applier struct {
	job

	apply func(aggregate.Aggregate)
}

// Errors returns an Option that provides a Stream with error channels. A Stream
// will cancel its operation as soon as an error can be received from one of the
// error channels.
func Errors[D any](errs ...<-chan error) Option[D] {
	return func(s *stream[D]) {
		s.streamErrors = append(s.streamErrors, errs...)
	}
}

// Sorted returns an Option that optimizes Aggregate builds by giving the
// Stream information about the order of incoming Events from the streams.New.
//
// When Sorted is disabled (which it is by default), the Stream sorts the
// collected Events for a specific Aggregate by the AggregateVersion of the
// Events before applying them to the Aggregate.
//
// Enable this option only if the underlying streams.New guarantees that
// incoming Events are sorted by AggregateVersion.
func Sorted[D any](v bool) Option[D] {
	return func(s *stream[D]) {
		s.isSorted = v
	}
}

// Grouped returns an Option that optimizes Aggregate builds by giving the
// Stream information about the order of incoming Events from the streams.New.
//
// When Grouped is disabled, the Stream has to wait for the streams.New to be
// drained before it can be sure no more Events will arrive for a specific
// Aggregate. When Grouped is enabled, the Stream knows when all Events for an
// Aggregate have been received and can therefore return the Aggregate as soon
// as its last Event has been received and applied.
//
// Grouped is disabled by default and should only be enabled if the correct
// order of events is guaranteed by the underlying streams.New. Events are
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
func Grouped[D any](v bool) Option[D] {
	return func(s *stream[D]) {
		s.isGrouped = v
	}
}

// ValidateConsistency returns an Option that optimizes Aggregate builds by
// controlling if the consistency of Events is validated before building an
// Aggregate from those Events.
//
// This option is enabled by default and should only be disabled if the
// consistency of Events is guaranteed by the underlying streams.New or if it's
// explicitly desired to put an Aggregate into an invalid state.
func ValidateConsistency[D any](v bool) Option[D] {
	return func(s *stream[D]) {
		s.validateConsistency = v
	}
}

// Filter returns an Option that filters incoming Events before they're handled
// by the Stream. Events are passed to every fn in fns until a fn returns false.
// If any of fns returns false, the Event is discarded by the Stream.
func Filter[D any](fns ...func(event.Of[D]) bool) Option[D] {
	return func(s *stream[D]) {
		s.filters = append(s.filters, fns...)
	}
}

// New takes a channel of Events and returns both a channel of Aggregate
// Histories and an error channel. A History apply itself on an Aggregate to
// build the current state of the Aggregate.
//
// Use the Drain function to get the Histories as a slice and a single error:
//
//	var events <-chan event.EventOf[D]
//	str, errs := stream.New(events)
//	histories, err := streams.Drain(context.TODO(), str, errs)
//	// handle err
//	for _, h := range histories {
//		foo := newFoo(h.AggregateID())
//		h.Apply(foo)
//	}
func New[D any, Events ~<-chan event.Of[D]](events Events, opts ...Option[D]) (<-chan aggregate.History, <-chan error) {
	if events == nil {
		evts := make(chan event.Of[D])
		close(evts)
		events = evts
	}

	aes := stream[D]{
		validateConsistency: true,
		stream:              events,
		acceptDone:          make(chan struct{}),
		events:              make(chan event.Of[D]),
		complete:            make(chan job),
		groupReqs:           make(chan groupRequest[D]),
		out:                 make(chan aggregate.History),
		outErrors:           make(chan error),
	}
	for _, opt := range opts {
		opt(&aes)
	}

	aes.inErrors, aes.stopErrors = streams.FanIn(aes.streamErrors...)

	go aes.acceptEvents()
	go aes.groupEvents()
	go aes.sortEvents()

	return aes.out, aes.outErrors
}

func (s *stream[D]) acceptEvents() {
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

func (s *stream[D]) shouldDiscard(evt event.Of[D]) bool {
	for _, fn := range s.filters {
		if !fn(evt) {
			return true
		}
	}
	return false
}

func (s *stream[D]) groupEvents() {
	groups := make(map[job][]event.Of[D])
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

func (s *stream[D]) sortEvents() {
	defer close(s.out)
	defer close(s.outErrors)
	defer close(s.groupReqs)

	for j := range s.complete {
		req := groupRequest[D]{
			job: j,
			out: make(chan []event.Of[D]),
		}
		s.groupReqs <- req
		events := <-req.out

		if !s.isSorted {
			events = event.Sort(events, event.SortAggregateVersion, event.SortAsc)
		}

		if s.validateConsistency {
			a := aggregate.New(j.name, j.id)
			if err := aggregate.ValidateConsistency[D](a, events); err != nil {
				s.outErrors <- err
				continue
			}
		}

		s.out <- applier{
			job: j,
			apply: func(a aggregate.Aggregate) {
				aggregate.ApplyHistory[D](a, events)
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
