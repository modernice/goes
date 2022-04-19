package projection

import (
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/helper/streams"
)

// ApplyOption is an option for Apply.
type ApplyOption func(*applyConfig)

type applyConfig struct {
	ignoreProgress bool
}

// IgnoreProgress returns an ApplyOption that makes Apply ignore the current
// progress of a projection so that it applies events to a projection even if
// an event's time is before the progress time of the projection.
func IgnoreProgress() ApplyOption {
	return func(cfg *applyConfig) {
		cfg.ignoreProgress = true
	}
}

// Apply applies events to the given projection.
//
// If the projection implements Guard, proj.GuardProjection(evt) is called for
// every event to determine if the event should be applied to the projection.
//
// If the projection implements ProgressAware, the time of the last applied
// event is applied to the projection by calling proj.SetProgress(evt).
func Apply(proj Target[any], events []event.Event, opts ...ApplyOption) {
	ApplyStream(proj, streams.New(events), opts...)
}

// ApplyStream applies events to the given projection.
//
// If the projection implements Guard, proj.GuardProjection(evt) is called for
// every event to determine if the event should be applied to the projection.
//
// If the projection implements ProgressAware, the time of the last applied
// event is applied to the projection by calling proj.SetProgress(evt).
func ApplyStream(target Target[any], events <-chan event.Event, opts ...ApplyOption) {
	cfg := newApplyConfig(opts...)

	progressor, isProgressor := target.(ProgressAware)
	guard, hasGuard := target.(Guard)

	var lastEventTime time.Time
	var lastEvents []uuid.UUID
	for evt := range events {
		if hasGuard && !guard.GuardProjection(evt) {
			continue
		}

		if isProgressor && !cfg.ignoreProgress && !progressorAllows(progressor, evt) {
			continue
		}

		target.ApplyEvent(evt)

		// Avoid unnecessary computations.
		if !isProgressor {
			continue
		}

		if !lastEventTime.IsZero() && lastEventTime.Equal(evt.Time()) {
			lastEvents = append(lastEvents, evt.ID())
			continue
		}

		lastEventTime = evt.Time()
		lastEvents = lastEvents[:0]
		lastEvents = append(lastEvents, evt.ID())
	}

	if isProgressor && !lastEventTime.IsZero() {
		progressor.SetProgress(lastEventTime, lastEvents...)
	}
}

func newApplyConfig(opts ...ApplyOption) applyConfig {
	var cfg applyConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}

func progressorAllows(progressor ProgressAware, evt event.Event) bool {
	progress, ids := progressor.Progress()

	if progress.IsZero() || progress.Before(evt.Time()) {
		return true
	}

	if progress.Unix() == evt.Time().Unix() {
		for _, id := range ids {
			if id == evt.ID() {
				return false
			}
		}
	}

	return true
}
