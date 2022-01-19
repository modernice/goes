package aggregate

//go:generate mockgen -source=aggregate.go -destination=./mocks/aggregate.go

import (
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/helper/pick"
	"github.com/modernice/goes/internal/xtime"
)

// Option is an Aggregate option.
type Option func(*Base)

// Base can be embedded into structs to make them implement Aggregate.
type Base struct {
	ID      uuid.UUID
	Name    string
	Version int
	Changes []event.Event

	handlers map[string]func(event.Event)
}

// Version returns an Option that sets the version of an Aggregate.
func Version(v int) Option {
	return func(b *Base) {
		b.Version = v
	}
}

type eventHandler interface {
	RegisterHandler(eventName string, handler func(event.Event))
}

// RegisterHandler registers an event handler for the given event name.
// The provided eventHandler should usually be an aggregate that uses the
// registered handler to apply the events onto itself. The *Base aggregate
// implements eventHandler.
//
//	type Foo struct {
//		*aggregate.Base
//
//		Foo string
//		Bar string
//		Baz string
//	}
//
//	type FooEvent { Foo string }
//	type BarEvent { Bar string }
//	type BazEvent { Bar string }
//
//	func NewFoo(id uuid.UUID) *Foo  {
//		foo := &Foo{Base: aggregate.New("foo", id)}
//		aggregate.Register(foo, "foo", foo.foo)
//		aggregate.Register(foo, "bar", foo.bar)
//		aggregate.Register(foo, "baz", foo.baz)
//		return foo
//	}
//
//	func (f *Foo) foo(e event.EventOf[FooEvent]) {
//		f.Foo = e.Data().Foo
//	}
//
//	func (f *Foo) foo(e event.EventOf[BarEvent]) {
//		f.Bar = e.Data().Bar
//	}
//
//	func (f *Foo) foo(e event.EventOf[BazEvent]) {
//		f.Baz = e.Data().Baz
//	}
func Register[D any](eh eventHandler, eventName string, handler func(event.EventOf[D])) {
	eh.RegisterHandler(eventName, func(evt event.Event) {
		if casted, ok := event.TryCast[D](evt); ok {
			handler(casted)
		} else {
			aggregateName := "<unknown>"
			if a, ok := eh.(Aggregate); ok {
				aggregateName = pick.AggregateName(a)
			}
			panic(fmt.Errorf(
				"[goes/aggregate.Register] Cannot cast %T to %T. "+
					"You probably provided the wrong event name for this handler. "+
					"[event=%v, aggregate=%v]",
				evt, casted, eventName, aggregateName,
			))
		}
	})
}

// RegisterHandler registers an event handler for the given event name.
// When b.ApplyEvent is called and a handler is registered for the given event,
// the provided handler is called.
//
// This method implements eventHandler.
func (b *Base) RegisterHandler(eventName string, handle func(event.Event)) {
	b.handlers[eventName] = handle
}

// New returns a new base aggregate.
func New(name string, id uuid.UUID, opts ...Option) *Base {
	b := &Base{
		ID:       id,
		Name:     name,
		handlers: make(map[string]func(event.EventOf[any])),
	}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

func (b *Base) Aggregate() (uuid.UUID, string, int) {
	return b.ID, b.Name, b.Version
}

// AggregateID implements Aggregate.
func (b *Base) AggregateID() uuid.UUID {
	return b.ID
}

// AggregateName implements Aggregate.
func (b *Base) AggregateName() string {
	return b.Name
}

// AggregateVersion implements Aggregate.
func (b *Base) AggregateVersion() int {
	return b.Version
}

// AggregateChanges implements Aggregate.
func (b *Base) AggregateChanges() []event.Event {
	return b.Changes
}

// TrackChange implements Aggregate.
func (b *Base) TrackChange(events ...event.Event) {
	b.Changes = append(b.Changes, events...)
}

// Commit implement Aggregate.
func (b *Base) Commit() {
	if len(b.Changes) == 0 {
		return
	}
	// b.TrackChange guarantees a correct event order, so we can safely assume
	// the last element has the highest version.
	b.Version = pick.AggregateVersion(b.Changes[len(b.Changes)-1])
	b.Changes = b.Changes[:0]
}

// ApplyEvent implements aggregate. Aggregates that embed *Base should override
// ApplyEvent.
func (b *Base) ApplyEvent(evt event.Event) {
	if handler, ok := b.handlers[evt.Name()]; ok {
		handler(evt)
	}
}

// SetVersion implements snapshot.Aggregate.
func (b *Base) SetVersion(v int) {
	b.Version = v
}

// ApplyHistory applies the given events to the aggregate a to reconstruct the
// state of a at the time of the latest event. If the aggregate implements
// Committer, a.TrackChange(events) and a.Commit() are called before returning.
func ApplyHistory[D any, E event.EventOf[D], Events ~[]E](a Aggregate, events Events) error {
	if err := ValidateConsistency[D, E, Events](a, events); err != nil {
		return fmt.Errorf("validate consistency: %w", err)
	}

	aevents := make([]event.Event, len(events))
	for i, evt := range events {
		aevt := event.Any[D](evt)
		aevents[i] = aevt
		a.ApplyEvent(aevt)
	}

	if c, ok := a.(Committer); ok {
		c.TrackChange(aevents...)
		c.Commit()
	}

	return nil
}

// Sort sorts aggregates and returns the sorted aggregates.
func Sort(as []Aggregate, s Sorting, dir SortDirection) []Aggregate {
	return SortMulti(as, SortOptions{Sort: s, Dir: dir})
}

// SortMulti sorts aggregates by multiple fields and returns the sorted
// aggregates.
func SortMulti(as []Aggregate, sorts ...SortOptions) []Aggregate {
	sorted := make([]Aggregate, len(as))
	copy(sorted, as)

	sort.Slice(sorted, func(i, j int) bool {
		for _, opts := range sorts {
			cmp := opts.Sort.Compare(sorted[i], sorted[j])
			if cmp != 0 {
				return opts.Dir.Bool(cmp < 0)
			}
		}
		return true
	})

	return sorted
}

// NextEvent makes and returns the next Event e for the aggregate a. NextEvent
// calls a.ApplyEvent(e) and a.TrackChange(e) before returning the Event.
//
//	var foo aggregate.Aggregate
//	evt := aggregate.NextEvent(foo, "event-name", ...)
func NextEvent[D any](a Aggregate, name string, data D, opts ...event.Option[D]) event.E[D] {
	aid, aname, _ := a.Aggregate()

	opts = append([]event.Option[D]{
		event.Aggregate[D](
			aid,
			aname,
			NextVersion(a),
		),
		event.Time[D](nextTime(a)),
	}, opts...)

	evt := event.New(name, data, opts...)
	aevt := evt.Any()

	a.ApplyEvent(aevt)

	if c, ok := a.(Committer); ok {
		c.TrackChange(aevt)
	}

	return evt
}

// HasChange returns whether Aggregate a has an uncommitted Event with the given name.
func HasChange(a Aggregate, eventName string) bool {
	for _, change := range a.AggregateChanges() {
		if change.Name() == eventName {
			return true
		}
	}
	return false
}

// UncommittedVersion returns the version the aggregate, including any uncommitted changes.
func UncommittedVersion(a Aggregate) int {
	_, _, v := a.Aggregate()
	return v + len(a.AggregateChanges())
}

// NextVersion returns the next (uncommitted) version of an aggregate (UncommittedVersion(a) + 1).
func NextVersion(a Aggregate) int {
	return UncommittedVersion(a) + 1
}

// nextTime returns the Time for the next event of the given aggregate. The time
// should most of the time just be time.Now(), but nextTime guarantees that the
// returned Time is at least 1 nanosecond after the previous event. This is
// necessary for the projection.Progressing API to work.
func nextTime(a Aggregate) time.Time {
	changes := a.AggregateChanges()
	now := xtime.Now()
	if len(changes) == 0 {
		return now
	}
	latestTime := changes[len(changes)-1].Time()
	nowTrunc := now.Truncate(0)
	if nowTrunc.Equal(latestTime) || nowTrunc.Before(latestTime.Truncate(0)) {
		return changes[len(changes)-1].Time().Add(time.Nanosecond)
	}
	return now
}
