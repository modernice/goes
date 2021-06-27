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

	// ErrProgressed is returned when trying to apply an Event onto a Projection
	// and the Event time is before the current progress time of the projection.
	ErrProgressed = errors.New("projection already progressed")
)

// A Projection is a projection of an event stream.
type Projection interface {
	ApplyEvent(event.Event)
}

// A Guard is an event query that determines which Events are allows to be
// applied onto a projection.
type Guard query.Query

// Progressor may be embedded into a projection to enable the projection to be
// resumed after the latest update to the projection.
type Progressor struct {
	LatestEventTime int64
}

// Base may be embedded into a projection type to provide sensible default
// behavior. Use New when instantiating Base.
type Base struct {
	*Progressor
}

// ApplyOption is an option for Apply.
type ApplyOption func(*applyConfig)

// IgnoreProgress returns an ApplyOption that makes Apply ignore the current
// progress of a projection so that it applies Events onto a projection even if
// an Event's time is before the progress time of the projection.
func IgnoreProgress() ApplyOption {
	return func(cfg *applyConfig) {
		cfg.ignoreProgress = true
	}
}

// Apply applies events onto proj.
//
// If proj implements guard (or embeds Guard), proj.GuardProjection(evt) is
// called for every Event evt to determine if the Event should be applied onto
// the Projection.
//
// If proj implements progressor (or embeds *Progressor), proj.SetProgress(evt)
// is called for every applied Event evt.
func Apply(proj Projection, events []event.Event, opts ...ApplyOption) error {
	cfg := newApplyConfig(opts...)

	progressor, isProgressor := proj.(progressor)
	guard, hasGuard := proj.(guard)

	for _, evt := range events {
		if hasGuard && !guard.GuardProjection(evt) {
			return fmt.Errorf("apply %q: %w", evt.Name(), ErrGuarded)
		}

		if isProgressor && !cfg.ignoreProgress {
			if progress := progressor.Progress(); !progress.IsZero() && !progress.Before(evt.Time()) {
				return fmt.Errorf("apply Event with time %v: %w", evt.Time(), ErrProgressed)
			}
		}

		proj.ApplyEvent(evt)

		if isProgressor && progressor.Progress().Before(evt.Time()) {
			progressor.SetProgress(evt.Time())
		}
	}

	return nil
}

// Progress returns the projection progress in terms of the time of the latest
// applied event.
func (p *Progressor) Progress() time.Time {
	return time.Unix(0, p.LatestEventTime)
}

// SetProgress sets the projection progress as the time of the latest applied event.
func (p *Progressor) SetProgress(t time.Time) {
	if t.IsZero() {
		p.LatestEventTime = 0
		return
	}
	p.LatestEventTime = t.UnixNano()
}

// New returns a new projection Base that can be embedded into a projection type
// to provide sensible default behavior.
func New() *Base {
	return &Base{Progressor: &Progressor{}}
}

// GuardProjection tests the Guard's Query against a given Event and returns
// whether the Event is allowed to be applied onto a projection.
func (g Guard) GuardProjection(evt event.Event) bool {
	return query.Test(query.Query(g), evt)
}

// ProjectionFilter returns a slice of event queries that can be used to query
// Events that are allowed by the Guard.
func (g Guard) ProjectionFilter() []event.Query {
	return []event.Query{query.Query(g)}
}

type progressor interface {
	Progress() time.Time
	SetProgress(time.Time)
}

type guard interface {
	GuardProjection(event.Event) bool
	ProjectionFilter() []event.Query
}

type resetter interface {
	Reset()
}

type applyConfig struct {
	ignoreProgress bool
}

func newApplyConfig(opts ...ApplyOption) applyConfig {
	var cfg applyConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}
