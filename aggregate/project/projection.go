package project

import (
	"github.com/google/uuid"
	"github.com/modernice/goes/event"
)

// A Projection projects an Aggregate on itself.
type Projection interface {
	// AggregateName returns the projected Aggregates name.
	AggregateName() string

	// AggregateID returns the projected Aggregates UUID.
	AggregateID() uuid.UUID

	// ApplyEvent applies the given Event on the Projection.
	ApplyEvent(evt event.Event)
}

type projection struct {
	aggregateName string
	aggregateID   uuid.UUID
}

// New returns a new Projection.
//
// Users shouldn't use New to create Projections but instead embed the
// Projection interface in their concrete structs and use New to instantiate the:
// embedded Projection:
//
//	type ProjectedFoo struct {
//		project.Projection
//	}
//
//	func NewProjectedFoo(id uuid.UUID) *ProjectedFoo {
//		return &ProjectedFoo{
//			Projection: project.New("foo", id),
//		}
//	}
func New(name string, id uuid.UUID) Projection {
	return &projection{
		aggregateName: name,
		aggregateID:   id,
	}
}

func (p *projection) AggregateName() string {
	return p.aggregateName
}

func (p *projection) AggregateID() uuid.UUID {
	return p.aggregateID
}

func (p *projection) ApplyEvent(evt event.Event) {}
