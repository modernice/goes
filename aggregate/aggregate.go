package aggregate

//go:generate mockgen -source=aggregate.go -destination=./mocks/aggregate.go

import (
	"fmt"
	"sort"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate/consistency"
	"github.com/modernice/goes/event"
)

// Aggregate is an event-sourced Aggregate.
type Aggregate interface {
	// AggregateID return the UUID of the Aggregate.
	AggregateID() uuid.UUID

	// AggregateName returns the name of the Aggregate.
	AggregateName() string

	// AggregateVersion returns the version of the Aggregate.
	AggregateVersion() int

	// AggregateChanges returns the uncommited changes (Events) of the Aggregate.
	AggregateChanges() []event.Event

	// TrackChange adds Events as changes to the Aggregate.
	TrackChange(...event.Event)

	// FlushChanges increases the version of the Aggregate by the number of
	// changes and empties the changes.
	FlushChanges()

	// ApplyEvent applies the Event on the Aggregate.
	ApplyEvent(event.Event)

	// SetVersion sets the version of the Aggregate (without uncommitted Events).
	SetVersion(int)
}

// Option is an Aggregate option.
type Option func(*Base)

// Base can be embedded into structs to make them fulfill the Aggregate interface.
type Base struct {
	ID      uuid.UUID
	Name    string
	Version int
	Changes []event.Event
}

// New returns a new Base for an Aggregate.
func New(name string, id uuid.UUID, opts ...Option) *Base {
	b := &Base{
		ID:   id,
		Name: name,
	}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

// Version returns an Option that sets the version of an Aggregate.
func Version(v int) Option {
	return func(b *Base) {
		b.Version = v
	}
}

// ApplyHistory applies the given events to the Aggregate a to reconstruct the
// state of a at the time of the latest event.
func ApplyHistory(a Aggregate, events ...event.Event) error {
	if err := consistency.Validate(a, events...); err != nil {
		return fmt.Errorf("validate consistency: %w", err)
	}
	for _, evt := range events {
		a.ApplyEvent(evt)
	}
	a.TrackChange(events...)
	a.FlushChanges()
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

// CurrentVersion returns the version of Aggregate a, including any uncommitted
// changes.
func CurrentVersion(a Aggregate) int {
	return a.AggregateVersion() + len(a.AggregateChanges())
}

// NextVersion returns the next version of the given Aggregate (CurrentVersion(a) + 1).
func NextVersion(a Aggregate) int {
	return CurrentVersion(a) + 1
}

// NextEvent makes and returns the next Event e for the given Aggregate. NextEvent
// calls a.ApplyEvent(e) and a.TrackChange(e) before returning the Event.
func NextEvent(a Aggregate, name string, data event.Data, opts ...event.Option) event.Event {
	opts = append([]event.Option{event.Aggregate(
		a.AggregateName(),
		a.AggregateID(), NextVersion(a),
	)}, opts...)
	evt := event.New(name, data, opts...)
	a.ApplyEvent(evt)
	a.TrackChange(evt)
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

// FlushChanges
func (b *Base) FlushChanges() {
	if len(b.Changes) == 0 {
		return
	}
	// b.TrackChange guarantees a correct event order, so we can safely assume
	// the last element has the highest version.
	b.Version = b.Changes[len(b.Changes)-1].AggregateVersion()
	b.Changes = b.Changes[:0]
}

// ApplyEvent implements Aggregate. Structs that embed Base should reimplement
// ApplyEvent accordingly.
func (*Base) ApplyEvent(event.Event) {}

// SetVersion implements Aggregate.
func (b *Base) SetVersion(v int) {
	b.Version = v
}
