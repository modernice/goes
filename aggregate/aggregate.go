package aggregate

import (
	"fmt"

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
	TrackChange(...event.Event) error
	// ApplyEvent applies the Event on the Aggregate.
	ApplyEvent(event.Event)
}

type base struct {
	id      uuid.UUID
	name    string
	version int
	changes []event.Event
}

// New returns a new base aggregate.
func New(name string, id uuid.UUID) Aggregate {
	return &base{
		id:   id,
		name: name,
	}
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

func (b *base) TrackChange(events ...event.Event) error {
	if err := consistency.Validate(b, events...); err != nil {
		return fmt.Errorf("validate consistency: %w", err)
	}
	b.changes = append(b.changes, events...)
	return nil
}

// ApplyEvent does nothing. Structs that embed base should implement ApplyEvent.
func (*base) ApplyEvent(event.Event) {}
