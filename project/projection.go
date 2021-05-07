package project

import (
	"time"

	"github.com/modernice/goes/event"
)

// An EventApplier applies Events onto itself in order to build the state of a projection.
type EventApplier interface {
	ApplyEvent(event.Event)
}

// Progressor can be embedded into a projection and enables the projection to
// be resumed the latest update that has been applied. A projection Job can
// optimize Event Queries for a specific projection that embeds a Progressor.
type Progressor struct {
	// LatestEvenTime is the unix nano time of the last applied Event.
	LatestEventTime int64
}

// TODO: should progressor be exported?
type progressor interface {
	ProgressProjection(time.Time)
	ProjectionProgress() time.Time
}

// TODO: should guard be exported?
type guard interface {
	GuardProjection(event.Event) bool
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
	if len(events) == 0 {
		return nil
	}

	guard, isGuard := proj.(guard)

	for _, evt := range events {
		if isGuard && !guard.GuardProjection(evt) {
			continue
		}

		proj.ApplyEvent(evt)
	}

	if p, ok := proj.(progressor); ok {
		latest := events[len(events)-1]
		p.ProgressProjection(latest.Time())
	}

	return nil
}

// ProgressProjection updates the projection progress to the provided Time.
func (p *Progressor) ProgressProjection(latestEventTime time.Time) {
	p.LatestEventTime = latestEventTime.UnixNano()
}

// ProjectionProgress returns the latest Event time of the projection.
func (p *Progressor) ProjectionProgress() time.Time {
	return time.Unix(0, p.LatestEventTime)
}
