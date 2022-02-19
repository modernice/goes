package stream

import (
	"fmt"

	"github.com/modernice/goes"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/helper/streams"
)

type Event[ID goes.ID] event.Of[any, ID]

// Option is a stream option.
type Option func(*options)

type options struct {
	isSorted            bool
	isGrouped           bool
	validateConsistency bool
	filters             []func(Event[goes.AID]) bool
	streamErrors        []<-chan error
}

type stream[ID goes.ID] struct {
	options

	stream     <-chan event.Of[any, ID]
	inErrors   <-chan error
	stopErrors func()

	acceptDone chan struct{}

	events   chan event.Of[any, ID]
	complete chan job[ID]

	groupReqs chan groupRequest[ID]

	out       chan aggregate.HistoryOf[ID]
	outErrors chan error
}

type job[ID goes.ID] struct {
	name string
	id   ID
}

type groupRequest[ID goes.ID] struct {
	job[ID]
	out chan []event.Of[any, ID]
}

type applier[ID goes.ID] struct {
	job[ID]

	apply func(aggregate.AggregateOf[ID])
}

// Errors returns an Option that provides a Stream with error channels. A Stream
// will cancel its operation as soon as an error can be received from one of the
// error channels.
func Errors(errs ...<-chan error) Option {
	return func(opts *options) {
		opts.streamErrors = append(opts.streamErrors, errs...)
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
func Sorted(v bool) Option {
	return func(opts *options) {
		opts.isSorted = v
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
func Grouped(v bool) Option {
	return func(opts *options) {
		opts.isGrouped = v
	}
}

// ValidateConsistency returns an Option that optimizes Aggregate builds by
// controlling if the consistency of Events is validated before building an
// Aggregate from those Events.
//
// This option is enabled by default and should only be disabled if the
// consistency of Events is guaranteed by the underlying streams.New or if it's
// explicitly desired to put an Aggregate into an invalid state.
func ValidateConsistency(v bool) Option {
	return func(opts *options) {
		opts.validateConsistency = v
	}
}

// Filter returns an Option that filters incoming Events before they're handled
// by the Stream. Events are passed to every fn in fns until a fn returns false.
// If any of fns returns false, the Event is discarded by the Stream.
func Filter[ID goes.ID](fns ...func(Event[ID]) bool) Option {
	return func(opts *options) {
		opts.filters = append(opts.filters, func(evt Event[goes.AID]) bool {
			e := event.MapID[ID, goes.AID, any](evt, goes.UnwrapID[ID, goes.AID])
			for _, fn := range fns {
				if !fn(e) {
					return false
				}
			}
			return true
		})
	}
}

// New takes a channel of Events and returns both a channel of Aggregate
// Histories and an error channel. A History apply itself on an Aggregate to
// build the current state of the Aggregate.
//
// Use the Drain function to get the Histories as a slice and a single error:
//
//	var events <-chan event.Of[any, uuid.UUID]
//	str, errs := stream.New(events)
//	histories, err := streams.Drain(context.TODO(), str, errs)
//	// handle err
//	for _, h := range histories {
//		foo := newFoo(h.AggregateID())
//		h.Apply(foo)
//	}
func New[ID goes.ID](events <-chan event.Of[any, ID], opts ...Option) (<-chan aggregate.HistoryOf[ID], <-chan error) {
	if events == nil {
		evts := make(chan event.Of[any, ID])
		close(evts)
		events = evts
	}

	aes := stream[ID]{
		options:    options{validateConsistency: true},
		stream:     events,
		acceptDone: make(chan struct{}),
		events:     make(chan event.Of[any, ID]),
		complete:   make(chan job[ID]),
		groupReqs:  make(chan groupRequest[ID]),
		out:        make(chan aggregate.HistoryOf[ID]),
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

func (s *stream[ID]) acceptEvents() {
	defer close(s.complete)
	defer close(s.acceptDone)
	defer close(s.events)
	defer s.stopErrors()

	pending := make(map[job[ID]]bool)

	var prev job[ID]
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

			j := job[ID]{
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

func (s *stream[ID]) shouldDiscard(evt event.Of[any, ID]) bool {
	e := event.AnyID(evt)

	for _, fn := range s.filters {
		if !fn(e) {
			return true
		}
	}

	return false
}

func (s *stream[ID]) groupEvents() {
	groups := make(map[job[ID]][]event.Of[any, ID])
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
			j := job[ID]{name, id}
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

func (s *stream[ID]) sortEvents() {
	defer close(s.out)
	defer close(s.outErrors)
	defer close(s.groupReqs)

	for j := range s.complete {
		req := groupRequest[ID]{
			job: j,
			out: make(chan []event.Of[any, ID]),
		}
		s.groupReqs <- req
		events := <-req.out

		if !s.isSorted {
			events = event.Sort(events, event.SortAggregateVersion, event.SortAsc)
		}

		if s.validateConsistency {
			a := aggregate.New(j.name, j.id)
			if err := aggregate.ValidateConsistency[ID](a, events); err != nil {
				s.outErrors <- err
				continue
			}
		}

		s.out <- applier[ID]{
			job:   j,
			apply: func(a aggregate.AggregateOf[ID]) { aggregate.ApplyHistory(a, events) },
		}
	}
}

func (a applier[ID]) AggregateName() string {
	return a.name
}

func (a applier[ID]) AggregateID() ID {
	return a.id
}

func (a applier[ID]) Apply(ag aggregate.AggregateOf[ID]) {
	a.apply(ag)
}
