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
// be resumed after the latest update to the projection. A projection Job can
// optimize Event Queries for a projection that embeds a Progressor by using its
// LatestEventTime to only query Events after that time.
//
// A projection that does not embed or implement Progressor cannot be resumed
// and must therefore be newly created for every projection Job to ensure that
// it doesn't apply an Event multiple times.
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
// most cases is a type that embeds a *Progressor.
//
// If proj provides a `Guard(event.Event) bool` method, that method is called
// for every Event before it is applied to determine if it should be applied.
// Guard must return true if the given Event should be applied.
//
// If proj provides a `ProgressProjection(time.Time)` method, that method is
// called for every applied Event with the time of that Event.
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
