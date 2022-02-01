package projection

import (
	"errors"
	"fmt"

	"github.com/modernice/goes/event"
)

var (
	// ErrProgressed is returned when trying to apply an Event onto a Projection
	// that has a progress Time that is after the Time of the Event.
	ErrProgressed = errors.New("projection already progressed")
)

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

// Apply applies events onto the given projection.
//
// If proj implements guard (or embeds Guard), proj.GuardProjection(evt) is
// called for every Event evt to determine if the Event should be applied onto
// the Projection.
//
// If proj implements progressor (or embeds *Progressor), proj.SetProgress(evt)
// is called for every applied Event evt.
func Apply(proj EventApplier, events []event.Event, opts ...ApplyOption) error {
	if len(events) == 0 {
		return nil
	}

	cfg := newApplyConfig(opts...)

	progressor, isProgressor := proj.(Progressing)
	guard, hasGuard := proj.(Guard)

	for _, evt := range events {
		if hasGuard && !guard.GuardProjection(evt) {
			continue
		}

		if isProgressor && !cfg.ignoreProgress {
			progress, version := progressor.Progress()
			if !progress.IsZero() && !progress.Before(evt.Time()) {
				return fmt.Errorf("apply event with time %v: %w", evt.Time(), ErrProgressed)
			}

			if evtVersion := event.PickAggregateVersion(evt); version > 0 && evtVersion <= version {
				return fmt.Errorf("apply event with version %v: %w", evtVersion, ErrProgressed)
			}
		}

		proj.ApplyEvent(evt)
	}

	if progress := events[len(events)-1].Time(); isProgressor {
		if currentProgress, _ := progressor.Progress(); currentProgress.Before(progress) {
			progressor.TrackProgress(progress, event.PickAggregateVersion(events[len(events)-1]))
		}
	}

	return nil
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
