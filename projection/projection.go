package projection

import (
	"errors"
	"fmt"
	"time"

	"github.com/modernice/goes/event"
)

var (
	// ErrProgressed is returned when trying to apply an Event onto a Projection
	// that has a progress Time that is after the Time of the Event.
	ErrProgressed = errors.New("projection already progressed")
)

// ApplyOption is an option for Apply.
type ApplyOption func(*applyConfig)

type applyConfig struct {
	ignoreProgress bool
}

// IgnoreProgress returns an ApplyOption that makes Apply ignore the current
// progress of a projection so that it applies Events onto a projection even if
// an Event's time is before the progress time of the projection.
func IgnoreProgress() ApplyOption {
	return func(cfg *applyConfig) {
		cfg.ignoreProgress = true
	}
}

// Apply applies events onto the given projection.
//
// If proj implements guard (or embeds Guard), proj.GuardProjection(evt) is
// called for every Event evt to determine if the Event should be applied onto
// the Projection.
//
// If proj implements progressor (or embeds *Progressor), proj.SetProgress(evt)
// is called for every applied Event evt.
func Apply[D any, Event event.Of[D], Events ~[]Event](proj EventApplier[D], events Events, opts ...ApplyOption) error {
	if len(events) == 0 {
		return nil
	}

	cfg := newApplyConfig(opts...)

	progressor, isProgressor := proj.(Progressing)
	guard, hasGuard := proj.(Guard[D])

	for _, evt := range events {
		if hasGuard && !guard.GuardProjection(evt) {
			continue
		}

		if isProgressor && !cfg.ignoreProgress {
			if progress := progressor.Progress(); !progress.IsZero() && !progress.Before(evt.Time()) {
				return fmt.Errorf("apply event with time %v: %w", evt.Time(), ErrProgressed)
			}
		}

		proj.ApplyEvent(evt)
	}

	if progress := events[len(events)-1].Time(); isProgressor && progressor.Progress().Before(progress) {
		progressor.SetProgress(progress)
	}

	return nil
}

// Progress returns the projection progress in terms of the time of the latest
// applied event. If p.LatestEventTime is 0, the zero Time is returned.
func (p *Progressor) Progress() time.Time {
	if p.LatestEventTime == 0 {
		return time.Time{}
	}
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

func newApplyConfig(opts ...ApplyOption) applyConfig {
	var cfg applyConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}
