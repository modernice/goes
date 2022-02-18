package aggregate

//go:generate mockgen -source=base.go -destination=./mocks/base.go

import (
	"fmt"
	"sort"
	"time"

	"github.com/modernice/goes"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/helper/pick"
	"github.com/modernice/goes/internal/xtime"
)

// Option is an Aggregate option.
type Option func(*Base[goes.AID])

// Base provides the basic implementation for aggregates.
type Base[ID goes.ID] struct {
	ID      ID
	Name    string
	Version int
	Changes []event.Of[any, ID]

	handlers map[string]func(event.Of[any, ID])
}

// Version returns an Option that sets the version of an Aggregate.
func Version(v int) Option {
	return func(opts *Base[goes.AID]) {
		opts.Version = v
	}
}

// New returns a new base aggregate.
func New[ID goes.ID](name string, id ID, opts ...Option) *Base[ID] {
	o := Base[goes.AID]{
		ID:       goes.AnyID(id),
		Name:     name,
		handlers: make(map[string]func(event.Of[any, goes.AID])),
	}
	for _, opt := range opts {
		opt(&o)
	}

	handlers := make(map[string]func(event.Of[any, ID]))
	for eventName, handler := range o.handlers {
		handlers[eventName] = func(evt event.Of[any, ID]) {
			handler(event.AnyID(evt))
		}
	}

	return &Base[ID]{
		ID:       o.ID.ID.(ID),
		Name:     o.Name,
		handlers: handlers,
		Version:  o.Version,
	}
}

// ApplyWith is an alias for event.RegisterHandler.
func ApplyWith[Data any, ID goes.ID, Event event.Of[Data, ID]](a event.Handler[ID], eventName string, apply func(Event)) {
	event.RegisterHandler[Data](a, eventName, apply)
}

// RegisterHandler registers an event handler for the given event name.
// When b.ApplyEvent is called and a handler is registered for the given event,
// the provided handler is called.
//
// This method implements event.Handler.
func (b *Base[ID]) RegisterHandler(eventName string, handler func(event.Of[any, ID])) {
	b.handlers[eventName] = handler
}

func (b *Base[ID]) Aggregate() (ID, string, int) {
	return b.ID, b.Name, b.Version
}

// AggregateID implements Aggregate.
func (b *Base[ID]) AggregateID() ID {
	return b.ID
}

// ModelID implements goes/persistence/model.Model.
func (b *Base[ID]) ModelID() ID {
	return b.ID
}

// AggregateName implements Aggregate.
func (b *Base[ID]) AggregateName() string {
	return b.Name
}

// AggregateVersion implements Aggregate.
func (b *Base[ID]) AggregateVersion() int {
	return b.Version
}

// AggregateChanges implements Aggregate.
func (b *Base[ID]) AggregateChanges() []event.Of[any, ID] {
	return b.Changes
}

// TrackChange implements Aggregate.
func (b *Base[ID]) TrackChange(events ...event.Of[any, ID]) {
	b.Changes = append(b.Changes, events...)
}

// Commit implement Aggregate.
func (b *Base[ID]) Commit() {
	if len(b.Changes) == 0 {
		return
	}
	// b.TrackChange guarantees a correct event order, so we can safely assume
	// the last element has the highest version.
	b.Version = pick.AggregateVersion[ID](b.Changes[len(b.Changes)-1])
	b.Changes = b.Changes[:0]
}

// ApplyEvent implements aggregate. Aggregates that embed *Base should override
// ApplyEvent.
func (b *Base[ID]) ApplyEvent(evt event.Of[any, ID]) {
	if handler, ok := b.handlers[evt.Name()]; ok {
		handler(evt)
	}
}

// SetVersion implements snapshot.Aggregate.
func (b *Base[ID]) SetVersion(v int) {
	b.Version = v
}

// ApplyHistory applies the given events to the aggregate a to reconstruct the
// state of a at the time of the latest event. If the aggregate implements
// Committer, a.TrackChange(events) and a.Commit() are called before returning.
func ApplyHistory[Data any, ID goes.ID, Events ~[]event.Of[Data, ID]](a AggregateOf[ID], events Events) error {
	if err := ValidateConsistency[ID, Data, Events](a, events); err != nil {
		return fmt.Errorf("validate consistency: %w", err)
	}

	aevents := make([]event.Of[any, ID], len(events))
	for i, evt := range events {
		aevt := event.ToAny(evt)
		aevents[i] = aevt
		a.ApplyEvent(aevt)
	}

	if c, ok := a.(Committer[ID]); ok {
		c.TrackChange(aevents...)
		c.Commit()
	}

	return nil
}

// Sort sorts aggregates and returns the sorted aggregates.
func Sort[ID goes.ID, Aggregates ~[]AggregateOf[ID]](as Aggregates, s Sorting, dir SortDirection) Aggregates {
	return SortMulti[ID, Aggregates](as, SortOptions{Sort: s, Dir: dir})
}

// SortMulti sorts aggregates by multiple fields and returns the sorted
// aggregates.
func SortMulti[ID goes.ID, Aggregates ~[]AggregateOf[ID]](as Aggregates, sorts ...SortOptions) Aggregates {
	sorted := make(Aggregates, len(as))
	copy(sorted, as)

	sort.Slice(sorted, func(i, j int) bool {
		for _, opts := range sorts {
			cmp := CompareSorting(opts.Sort, sorted[i], sorted[j])
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
func NextEvent[ID goes.ID, Data any](a AggregateOf[ID], id ID, name string, data Data, opts ...event.Option) event.E[Data, ID] {
	aid, aname, _ := a.Aggregate()

	opts = append([]event.Option{
		event.Aggregate(
			aid,
			aname,
			NextVersion(a),
		),
		event.Time(nextTime(a)),
	}, opts...)

	evt := event.New(id, name, data, opts...)
	aevt := evt.Any()

	a.ApplyEvent(aevt)

	if c, ok := a.(Committer[ID]); ok {
		c.TrackChange(aevt)
	}

	return evt
}

// HasChange returns whether Aggregate a has an uncommitted Event with the given name.
func HasChange[ID goes.ID](a AggregateOf[ID], eventName string) bool {
	for _, change := range a.AggregateChanges() {
		if change.Name() == eventName {
			return true
		}
	}
	return false
}

// UncommittedVersion returns the version the aggregate, including any uncommitted changes.
func UncommittedVersion[ID goes.ID](a AggregateOf[ID]) int {
	_, _, v := a.Aggregate()
	return v + len(a.AggregateChanges())
}

// NextVersion returns the next (uncommitted) version of an aggregate (UncommittedVersion(a) + 1).
func NextVersion[ID goes.ID](a AggregateOf[ID]) int {
	return UncommittedVersion(a) + 1
}

// nextTime returns the Time for the next event of the given aggregate. The time
// should most of the time just be time.Now(), but nextTime guarantees that the
// returned Time is at least 1 nanosecond after the previous event. This is
// necessary for the projection.Progressing API to work.
func nextTime[ID goes.ID](a AggregateOf[ID]) time.Time {
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
