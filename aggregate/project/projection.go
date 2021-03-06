package project

import (
	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
)

// A Projection projects an Aggregate on itself.
type Projection interface {
	aggregate.Aggregate
}

type projection struct {
	aggregate.Aggregate
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
		Aggregate: aggregate.New(name, id),
	}
}
