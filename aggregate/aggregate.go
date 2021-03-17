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
	// AggregateID return the UUID of the aggregate.
	AggregateID() uuid.UUID

	// AggregateName returns the name of the aggregate.
	AggregateName() string

	// AggregateVersion returns the version of the aggregate.
	AggregateVersion() int

	// AggregateChanges returns the uncommited Events of the Aggregate.
	AggregateChanges() []event.Event

	// TrackChange adds the Events to the changes of the Aggregate.
	TrackChange(...event.Event)

	// FlushChanges increases the version of the Aggregate by the number of
	// changes and empties the changes.
	FlushChanges()

	// ApplyEvent applies the Event on the Aggregate.
	ApplyEvent(event.Event)

	// SetVersion sets the version of the Aggregate (without uncommitted Events).
	SetVersion(int)
}

// Option is an aggregate option.
type Option func(*base)

type base struct {
	id      uuid.UUID
	name    string
	version int
	changes []event.Event
}

// New returns a new base aggregate.
func New(name string, id uuid.UUID, opts ...Option) Aggregate {
	b := &base{
		id:   id,
		name: name,
	}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

// Version returns an Option that sets the version of an Aggregate.
func Version(v int) Option {
	return func(b *base) {
		b.version = v
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
func NextEvent(a Aggregate, name string, data event.Data) event.Event {
	evt := event.New(name, data, event.Aggregate(
		a.AggregateName(),
		a.AggregateID(),
		NextVersion(a),
	))
	a.ApplyEvent(evt)
	a.TrackChange(evt)
	return evt
}

func (b *base) AggregateID() uuid.UUID {
	return b.id
}

func (b *base) AggregateName() string {
	return b.name
}

func (b *base) AggregateVersion() int {
	return b.version
}

func (b *base) AggregateChanges() []event.Event {
	return b.changes
}

func (b *base) TrackChange(events ...event.Event) {
	b.changes = append(b.changes, events...)
}

func (b *base) FlushChanges() {
	if len(b.changes) == 0 {
		return
	}
	// b.TrackChange guarantees a correct event order, so we can safely assume
	// the last element has the highest version.
	b.version = b.changes[len(b.changes)-1].AggregateVersion()
	b.changes = b.changes[:0]
}

// ApplyEvent does nothing. Structs that embed base should implement ApplyEvent.
func (*base) ApplyEvent(event.Event) {}

// SetVersion sets the base version (version without uncommitted Events) of the
// Aggregate.
func (b *base) SetVersion(v int) {
	b.version = v
}
