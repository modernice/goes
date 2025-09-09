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

// Option configures a [Base].
type Option func(*Base)

// Base provides common state and helpers for aggregates. When embedded it
// satisfies [Aggregate], [Committer], repository.ChangeDiscarder and
// snapshot.Aggregate.
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

// Version sets the initial version of a Base.
func Version(v int) Option {
	return func(b *Base) { b.Version = v }
}

// New returns a Base for the given name and id. Options may modify the Base
// before it is returned.
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

// Ref returns the aggregate reference.
func (b *Base) Ref() Ref {
	return Ref{Name: b.Name, ID: b.ID}
}

// ModelID returns the aggregate id.
func (b *Base) ModelID() uuid.UUID { return b.ID }

// Aggregate implements [Aggregate].
func (b *Base) Aggregate() (uuid.UUID, string, int) {
	return b.ID, b.Name, b.Version
}

// AggregateID returns the aggregate id.
func (b *Base) AggregateID() uuid.UUID { return b.ID }

// AggregateName returns the aggregate name.
func (b *Base) AggregateName() string { return b.Name }

// AggregateVersion returns the committed version.
func (b *Base) AggregateVersion() int { return b.Version }

// CurrentVersion returns the version after applying uncommitted changes.
func (b *Base) CurrentVersion() int {
	return b.AggregateVersion() + len(b.AggregateChanges())
}

// AggregateChanges lists uncommitted events.
func (b *Base) AggregateChanges() []event.Event { return b.Changes }

// RecordChange appends events to the change list.
func (b *Base) RecordChange(events ...event.Event) {
	b.Changes = append(b.Changes, events...)
}

// Commit advances the version to the last change and clears the change list.
func (b *Base) Commit() {
	if len(b.Changes) == 0 {
		return
	}
	b.Version = pick.AggregateVersion(b.Changes[len(b.Changes)-1])
	b.Changes = b.Changes[:0]
}

// DiscardChanges drops all uncommitted events.
func (b *Base) DiscardChanges() { b.Changes = b.Changes[:0] }

// ApplyEvent dispatches evt to registered handlers.
func (b *Base) ApplyEvent(evt event.Event) { b.eventHandlers.HandleEvent(evt) }

// SetVersion sets the aggregate version. It implements snapshot.Aggregate.
func (b *Base) SetVersion(v int) { b.Version = v }

// Sort sorts aggregates by the given field and direction.
func Sort(as []Aggregate, s Sorting, dir SortDirection) []Aggregate {
	return SortMulti(as, SortOptions{Sort: s, Dir: dir})
}

// SortMulti sorts aggregates by multiple sort options.
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

// Deprecated: use Next instead.
func NextEvent[D any](a Aggregate, name string, data D, opts ...event.Option) event.Evt[D] {
	return Next(a, name, data, opts...)
}

// Next creates an event for a and applies it. If a implements [Committer] the
// event is recorded as an uncommitted change. Version and time are set
// automatically.
func Next[Data any](a Aggregate, name string, data Data, opts ...event.Option) event.Evt[Data] {
	aid, aname, _ := a.Aggregate()
	opts = append([]event.Option{
		event.Aggregate(aid, aname, NextVersion(a)),
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

// UncommittedVersion returns the version after applying uncommitted events.
func UncommittedVersion(a Aggregate) int {
	_, _, v := a.Aggregate()
	if changes := a.AggregateChanges(); len(changes) > 0 {
		if ev := pick.AggregateVersion(changes[len(changes)-1]); ev > v {
			return ev
		}
	}
	return v
}

// NextVersion returns the next event version for a.
func NextVersion(a Aggregate) int { return UncommittedVersion(a) + 1 }

// nextTime returns a timestamp for the next event, ensuring it is after the last
// change.
func nextTime(a Aggregate) time.Time {
	changes := a.AggregateChanges()
	now := xtime.Now()
	if len(changes) == 0 {
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
