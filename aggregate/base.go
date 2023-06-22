package aggregate

import (
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/command"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/helper/pick"
	"github.com/modernice/goes/internal/xtime"
)

// Option is a function that configures a [Base] aggregate. It is used as an
// argument in the New function to customize the created aggregate.
type Option func(*Base)

// Base provides the core of an event-sourced aggregate.
// When embedded into an aggregate, the aggregate will implement these APIs:
//   - aggregate.Aggregate
//   - aggregate.Committer
//   - repository.ChangeDiscarder
//   - snapshot.Aggregate
type Base struct {
	ID      uuid.UUID
	Name    string
	Version int
	Changes []event.Event

	eventHandlers
	commandHandlers
}

type eventHandlers = event.Handlers
type commandHandlers = command.Handlers

// Version is an Option that sets the Version of the Base struct when creating a
// new aggregate with the New function.
func Version(v int) Option {
	return func(b *Base) {
		b.Version = v
	}
}

// New creates a new Base aggregate with the specified name and UUID, applying
// the provided options. The returned Base can be embedded into custom
// aggregates to provide core functionality for event-sourced aggregates.
func New(name string, id uuid.UUID, opts ...Option) *Base {
	b := &Base{
		ID:              id,
		Name:            name,
		eventHandlers:   make(eventHandlers),
		commandHandlers: make(commandHandlers),
	}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

// Ref returns a Ref object containing the Name and ID of the aggregate.
func (b *Base) Ref() Ref {
	return Ref{
		Name: b.Name,
		ID:   b.ID,
	}
}

// ModelID returns the UUID of the aggregate that the Base is embedded into.
func (b *Base) ModelID() uuid.UUID {
	return b.ID
}

// Aggregate returns the ID, name, and version of an event-sourced aggregate. It
// is used to retrieve information about the aggregate without accessing its
// fields directly.
func (b *Base) Aggregate() (uuid.UUID, string, int) {
	return b.ID, b.Name, b.Version
}

// AggregateID returns the UUID of the aggregate associated with the Base
// struct.
func (b *Base) AggregateID() uuid.UUID {
	return b.ID
}

// AggregateName returns the name of the aggregate.
func (b *Base) AggregateName() string {
	return b.Name
}

// AggregateVersion returns the current version of the aggregate. The version is
// incremented when events are committed to the aggregate.
func (b *Base) AggregateVersion() int {
	return b.Version
}

// CurrentVersion returns the current version of the aggregate, which is the sum
// of its base version and the number of uncommitted changes.
func (b *Base) CurrentVersion() int {
	return b.AggregateVersion() + len(b.AggregateChanges())
}

// AggregateChanges returns the uncommitted changes (events) of the
// event-sourced aggregate. These are the events that have been recorded but not
// yet committed to the event store.
func (b *Base) AggregateChanges() []event.Event {
	return b.Changes
}

// RecordChange appends the provided events to the Changes slice of the Base
// aggregate.
func (b *Base) RecordChange(events ...event.Event) {
	b.Changes = append(b.Changes, events...)
}

// Commit updates the aggregate version to the version of its latest change and
// clears the changes. If there are no changes, nothing is done.
func (b *Base) Commit() {
	if len(b.Changes) == 0 {
		return
	}
	b.Version = pick.AggregateVersion(b.Changes[len(b.Changes)-1])
	b.Changes = b.Changes[:0]
}

// DiscardChanges resets the list of recorded changes to an empty state,
// effectively discarding any uncommitted changes made to the aggregate.
func (b *Base) DiscardChanges() {
	b.Changes = b.Changes[:0]
}

// ApplyEvent applies the given event to the aggregate by calling the
// appropriate event handler registered for the event's name. The event must
// have been created with the aggregate's ID, name, and version.
func (b *Base) ApplyEvent(evt event.Event) {
	b.eventHandlers.HandleEvent(evt)
}

// SetVersion manually sets the version of the aggregate.
//
// SetVersion implements snapshot.Aggregate.
func (b *Base) SetVersion(v int) {
	b.Version = v
}

// Sort sorts the given Aggregates ([]Aggregate) according to the specified
// Sorting and SortDirection. The sorted Aggregates are returned as a new slice
// without modifying the input slice.
func Sort(as []Aggregate, s Sorting, dir SortDirection) []Aggregate {
	return SortMulti(as, SortOptions{Sort: s, Dir: dir})
}

// SortMulti sorts a slice of Aggregates by multiple SortOptions in the order
// they are provided. If two Aggregates have the same value for a SortOption,
// the next SortOption in the list is used to determine their order.
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

// Deprecated: Use Next instead.
func NextEvent[D any](a Aggregate, name string, data D, opts ...event.Option) event.Evt[D] {
	return Next(a, name, data, opts...)
}

// Next creates a new event with the provided name and data, applies it to the
// given aggregate, and records the change if the aggregate implements the
// Committer interface. The event is assigned the next available version and a
// timestamp that is guaranteed to be at least 1 nanosecond after the previous
// event.
func Next[Data any](a Aggregate, name string, data Data, opts ...event.Option) event.Evt[Data] {
	aid, aname, _ := a.Aggregate()

	opts = append([]event.Option{
		event.Aggregate(
			aid,
			aname,
			NextVersion(a),
		),
		event.Time(nextTime(a)),
	}, opts...)

	evt := event.New(name, data, opts...)
	aevt := evt.Any()

	a.ApplyEvent(aevt)

	if c, ok := a.(Committer); ok {
		c.RecordChange(aevt)
	}

	return evt
}

// UncommittedVersion returns the version of the given Aggregate after applying
// all uncommitted changes. It takes into account both the current version and
// any uncommitted events to calculate the resulting version.
func UncommittedVersion(a Aggregate) int {
	_, _, v := a.Aggregate()
	if changes := a.AggregateChanges(); len(changes) > 0 {
		if ev := pick.AggregateVersion(changes[len(changes)-1]); ev > v {
			return ev
		}
	}
	return v
}

// NextVersion returns the next version number for the given Aggregate, taking
// into account both its committed and uncommitted changes.
func NextVersion(a Aggregate) int {
	return UncommittedVersion(a) + 1
}

// nextTime returns the Time for the next event of the given aggregate. The time
// should most of the time just be time.Now(), but nextTime guarantees that the
// returned Time is at least 1 nanosecond after the previous event.
func nextTime(a Aggregate) time.Time {
	changes := a.AggregateChanges()
	now := xtime.Now()

	if len(changes) == 0 {
		// If the machine does not support nanosecond precision, and the
		// aggregate has no prior uncommited changes, we cannot check that the
		// time is at least 1 nanosecond after the previous event. In this case,
		// we add 1 microsecond to the time and sleep until that time.
		if !xtime.SupportsNanoseconds() && len(a.AggregateChanges()) == 0 {
			out := now.Add(time.Microsecond)
			time.Sleep(time.Until(out))
			return out
		}

		return now
	}

	latestTime := changes[len(changes)-1].Time()
	latestTimeTrunc := latestTime.Truncate(0)
	nowTrunc := now.Truncate(0)

	if nowTrunc.Equal(latestTimeTrunc) || nowTrunc.Before(latestTimeTrunc) {
		out := changes[len(changes)-1].Time().Add(time.Nanosecond)
		time.Sleep(time.Until(out))
		return out
	}

	return now
}
