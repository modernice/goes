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

// Option is an option for creating an aggregate.
type Option func(*Base)

// Base can be embedded into aggregates to implement the goes' APIs:
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

// Version returns an Option that sets the version of an aggregate.
func Version(v int) Option {
	return func(b *Base) {
		b.Version = v
	}
}

// New returns a new base aggregate.
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

// Ref returns a Ref to the given aggregate.
func (b *Base) Ref() Ref {
	return Ref{
		Name: b.Name,
		ID:   b.ID,
	}
}

// ModelID implements goes/persistence/model.Model. This allows *Base to be used
// as a TypedAggregate for the type parameter of a TypedRepository.
func (b *Base) ModelID() uuid.UUID {
	return b.ID
}

// Aggregate retrns the id, name, and version of the aggregate.
func (b *Base) Aggregate() (uuid.UUID, string, int) {
	return b.ID, b.Name, b.Version
}

// AggregateID returns the aggregate id.
func (b *Base) AggregateID() uuid.UUID {
	return b.ID
}

// AggregateName returns the aggregate name.
func (b *Base) AggregateName() string {
	return b.Name
}

// AggregateVersion returns the aggregate version.
func (b *Base) AggregateVersion() int {
	return b.Version
}

// CurrentVersion returns the version of the aggregate with respect to the
// uncommitted changes/events.
func (b *Base) CurrentVersion() int {
	return b.AggregateVersion() + len(b.AggregateChanges())
}

// AggregateChanges returns the recorded changes.
func (b *Base) AggregateChanges() []event.Event {
	return b.Changes
}

// RecordChange records applied changes to the aggregate.
func (b *Base) RecordChange(events ...event.Event) {
	b.Changes = append(b.Changes, events...)
}

// Commit clears the recorded changes and sets the aggregate version to the
// version of the last recorded change. The recorded changes must be sorted by
// event version.
func (b *Base) Commit() {
	if len(b.Changes) == 0 {
		return
	}
	b.Version = pick.AggregateVersion(b.Changes[len(b.Changes)-1])
	b.Changes = b.Changes[:0]
}

// DiscardChanges discards the recorded changes. The aggregate repository calls
// this method when retrying a failed Repository.Use() call. Note that this
// method does not discard any state changs that were applied to the aggregate;
// it only discards recorded changes.
func (b *Base) DiscardChanges() {
	b.Changes = b.Changes[:0]
}

// ApplyEvent calls the registered event appliers for the given event.
func (b *Base) ApplyEvent(evt event.Event) {
	b.eventHandlers.HandleEvent(evt)
}

// SetVersion manually sets the version of the aggregate.
//
// SetVersion implements snapshot.Aggregate.
func (b *Base) SetVersion(v int) {
	b.Version = v
}

// Sort sorts aggregates and returns the sorted aggregates.
func Sort(as []Aggregate, s Sorting, dir SortDirection) []Aggregate {
	return SortMulti(as, SortOptions{Sort: s, Dir: dir})
}

// SortMulti sorts aggregates by multiple fields and returns the sorted aggregates.
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

// Next creates, applies and returns the next event for the given aggregate.
//
//	var foo aggregate.Aggregate
//	evt := aggregate.Next(foo, "name", <data>, ...)
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

// UncommittedVersion returns the version of the aggregate after committing the
// recorded changes.
func UncommittedVersion(a Aggregate) int {
	_, _, v := a.Aggregate()
	if changes := a.AggregateChanges(); len(changes) > 0 {
		if ev := pick.AggregateVersion(changes[len(changes)-1]); ev > v {
			return ev
		}
	}
	return v
}

// NextVersion returns the version that the next event of the aggregate must have.
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
