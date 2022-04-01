package projection

import (
	"errors"
	"fmt"

	"github.com/modernice/goes/event"
	"github.com/modernice/goes/helper/streams"
)

var (
	// ErrProgressed is returned when trying to apply an event onto a projection
	// that has a progress Time that is after the Time of the event.
	ErrProgressed = errors.New("projection already progressed")
)

// ApplyOption is an option for Apply.
type ApplyOption func(*applyConfig)

type applyConfig struct {
	ignoreProgress bool
}

// IgnoreProgress returns an ApplyOption that makes Apply ignore the current
// progress of a projection so that it applies events onto a projection even if
// an event's time is before the progress time of the projection.
func IgnoreProgress() ApplyOption {
	return func(cfg *applyConfig) {
		cfg.ignoreProgress = true
	}
}

// Apply applies events onto the given projection.
//
// If the projection implements Guard, proj.GuardProjection(evt) is/ called for
// every event to determine if the event should be applied onto the projection.
//
// If the projection implements ProgressAware, the time of the last applied
// event is applied to the projection by calling proj.SetProgress(evt).
func Apply(proj EventApplier[any], events []event.Event, opts ...ApplyOption) error {
	return ApplyStream(proj, streams.New(events...), opts...)
}

// ApplyStream applies events onto the given projection.
//
// If the projection implements Guard, proj.GuardProjection(evt) is/ called for
// every event to determine if the event should be applied onto the projection.
//
// If the projection implements ProgressAware, the time of the last applied
// event is applied to the projection by calling proj.SetProgress(evt).
func ApplyStream(proj EventApplier[any], events <-chan event.Event, opts ...ApplyOption) error {
	cfg := newApplyConfig(opts...)

	progressor, isProgressor := proj.(ProgressAware)
	guard, hasGuard := proj.(Guard)

	var lastEvent event.Event
	for evt := range events {
		if hasGuard && !guard.GuardProjection(evt) {
			continue
		}

		if isProgressor && !cfg.ignoreProgress {
			if progress := progressor.Progress(); !progress.IsZero() && !progress.Before(evt.Time()) {
				return fmt.Errorf("apply event with time %v: %w", evt.Time(), ErrProgressed)
			}
		}

		proj.ApplyEvent(evt)
		lastEvent = evt
	}

	if isProgressor && lastEvent != nil {
		if progress := lastEvent.Time(); progressor.Progress().Before(progress) {
			progressor.SetProgress(progress)
		}
	}

	return nil
}

func newApplyConfig(opts ...ApplyOption) applyConfig {
	var cfg applyConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}
