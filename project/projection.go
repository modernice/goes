package project

import (
	"time"

	"github.com/modernice/goes/event"
)

// Projection can be embedded into structs so that they can be used as projections.
//
// Example:
//	type Example struct {
//		*project.Projection
//	}
//
//	func NewExample() *Example {
//		return &Example{
//			Projection: project.NewProjection()
//		}
//	}
type Projection struct {
	latest time.Time
}

// NewProjection returns a Projection that can be embedded into a struct.
func NewProjection() *Projection {
	return &Projection{}
}

// PostApplyEvent is called by a Projector after the given Event has been
// applied and sets the LatestEventTime to evt.Time().
func (p *Projection) PostApplyEvent(evt event.Event) {
	if evt != nil {
		p.latest = evt.Time()
	}
}

// LatestEventTime returns the time of the latest applied Event.
func (p *Projection) LatestEventTime() time.Time {
	return p.latest
}

// ApplyEvent implements EventApplier. Structs that embed Projection should
// reimplement ApplyEvent.
func (p *Projection) ApplyEvent(event.Event) {}
