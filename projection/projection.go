package projection

import (
	"errors"
	"fmt"
	"time"

	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/query"
)

var (
	// ErrGuarded is returned when trying to apply an Event onto a Projection
	// which has a guard that doesn't allow the Event.
	ErrGuarded = errors.New("guarded projection")
)

// A Projection is a projection of an event stream.
type Projection interface {
	ApplyEvent(event.Event)
}

type progressor interface {
	Progress() time.Time
	SetProgress(time.Time)
}

type guard interface {
	GuardProjection(event.Event) bool
}

// Apply applies events onto proj.
//
// If proj implements guard (or embeds Guard), proj.GuardProjection(evt) is
// called for every Event evt to determine if the Event should be applied onto
// the Projection.
//
// If proj implements progressor (or embeds *Progressor), proj.SetProgress(evt)
// is called for every applied Event evt.
func Apply(proj Projection, events []event.Event) error {
	progressor, isProgressor := proj.(progressor)
	guard, hasGuard := proj.(guard)

	for _, evt := range events {
		if hasGuard && !guard.GuardProjection(evt) {
			return fmt.Errorf("apply %q: %w", evt.Name(), ErrGuarded)
		}

		proj.ApplyEvent(evt)

		if isProgressor {
			progressor.SetProgress(evt.Time())
		}
	}

	return nil
}

// Progressor may be embedded into a projection to enable the projection to be
// resumed after the latest update to the projection.
type Progressor struct {
	LatestEventTime int64
}

// Progress returns the projection progress in terms of the time of the latest
// applied event.
func (p *Progressor) Progress() time.Time {
	return time.Unix(0, p.LatestEventTime)
}

// SetProgress sets the projection progress as the time of the latest applied event.
func (p *Progressor) SetProgress(t time.Time) {
	p.LatestEventTime = t.UnixNano()
}

// Base may be embedded into a projection type to provide sensible default
// behavior. Use New when instantiating Base.
type Base struct {
	*Progressor
}

// New returns a new projection Base that can be embedded into a projection type
// to provide sensible default behavior.
func New() *Base {
	return &Base{Progressor: &Progressor{}}
}

// A Guard is an event query that determines which Events are allows to be
// applied onto a projection.
type Guard query.Query

// GuardProjection tests the Guard's Query against a given Event and returns
// whether the Event is allowed to be applied onto a projection.
func (g Guard) GuardProjection(evt event.Event) bool {
	return query.Test(query.Query(g), evt)
}
