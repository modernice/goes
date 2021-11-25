package projection

import (
	"time"

	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/query"
)

// Progressing makes projections track their projection progress.
//
// Embed *Progressor into a projection type to implement this interface.
//
// The current progress of a projection is the Time of the last applied event.
// A projection that provides its projection progress only receives events with
// a Time that is after the current progress Time.
type Progressing interface {
	// Progress returns the projection's progress as the Time of the last
	// applied event.
	Progress() time.Time

	// SetProgress sets the progress of the projection to the provided Time.
	SetProgress(time.Time)
}

// Progressor can be embedded into a projection to implement the Progressing interface.
type Progressor struct {
	LatestEventTime int64
}

// A Resetter is a projection that can reset its state.
type Resetter interface {
	// Reset should implement any custom logic to reset the state of a
	// projection besides resetting the progress (if the projection implements
	// Progressing).
	Reset()
}

// Guard can be implemented by projection to prevent the application of an
// events. When a projection p implements Guard, p.GuardProjection(e) is called
// for every Event e and prevents the p.ApplyEvent(e) call if GuardProjection
// returns false.
type Guard interface {
	// GuardProjection determines whether an Event is allowed to be applied onto a projection.
	GuardProjection(event.Event) bool
}

// A QueryGuard is an event query that determines which Events are allows to be
// applied onto a projection.
type QueryGuard query.Query

// GuardFunc allows functions to be used as Guards.
type GuardFunc func(event.Event) bool

// GuardProjection returns guard(evt).
func (guard GuardFunc) GuardProjection(evt event.Event) bool {
	return guard(evt)
}

// GuardProjection tests the Guard's Query against a given Event and returns
// whether the Event is allowed to be applied onto a projection.
func (g QueryGuard) GuardProjection(evt event.Event) bool {
	return query.Test(query.Query(g), evt)
}

// ProjectionFilter returns the Query in a slice.
func (g QueryGuard) ProjectionFilter() []event.Query {
	return []event.Query{query.Query(g)}
}
