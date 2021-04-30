package project

import (
	"time"

	"github.com/modernice/goes/event"
)

// An EventApplier applies Events onto itself in order to build the state of a projection.
type EventApplier interface {
	ApplyEvent(event.Event)
}

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
	LatestEventAppliedAt time.Time
}

type latestEventTimeProvider interface {
	LatestEventTime() time.Time
}

type postEventApplier interface {
	PostApplyEvent(event.Event)
}

type guard interface {
	Guard(event.Event) bool
}

// NewProjection returns a Projection that can be embedded into a struct.
func NewProjection() *Projection {
	return &Projection{}
}

// Apply projects the provided Events onto the given EventApplier, which in
// most cases is a type that embeds a *Projection.
//
// If proj provides a `Guard(event.Event) bool` method, that method is called
// with Events before they are applied to determine if they should be applied.
// Guard must return true if the given Event should be applied.
//
// If proj provides a `PostApplyEvent(event.Event)` method, that method is
// called after an Event has been applied with that Event as the argument.
func Apply(events []event.Event, proj EventApplier) error {
	postApplier, hasPostApply := proj.(postEventApplier)
	guard, isGuard := proj.(guard)

	for _, evt := range events {
		if isGuard && !guard.Guard(evt) {
			continue
		}

		proj.ApplyEvent(evt)

		if hasPostApply {
			postApplier.PostApplyEvent(evt)
		}
	}

	return nil
}

// PostApplyEvent is called by a Projector after the given Event has been
// applied and sets the LatestEventTime to evt.Time().
func (p *Projection) PostApplyEvent(evt event.Event) {
	if evt != nil && evt.Time().After(p.LatestEventAppliedAt) {
		p.LatestEventAppliedAt = evt.Time()
	}
}

// LatestEventTime returns the time of the latest applied Event.
func (p *Projection) LatestEventTime() time.Time {
	return p.LatestEventAppliedAt
}

// ApplyEvent implements EventApplier. Structs that embed Projection should
// reimplement ApplyEvent.
func (p *Projection) ApplyEvent(event.Event) {}
