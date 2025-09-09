package projection

import (
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/helper/streams"
)

// ApplyOption configures [Apply] and [ApplyStream].
type ApplyOption func(*applyConfig)

type applyConfig struct {
	ignoreProgress bool
}

// IgnoreProgress forces application of events even if they are older than the
// recorded progress.
func IgnoreProgress() ApplyOption {
	return func(cfg *applyConfig) {
		cfg.ignoreProgress = true
	}
}

// Apply feeds a slice of events to a projection. Guards can reject events and
// [ProgressAware] targets record the last applied time and ids.
func Apply(proj Target[any], events []event.Event, opts ...ApplyOption) {
	ApplyStream(proj, streams.New(events), opts...)
}

// ApplyStream feeds a stream of events to a projection with the same semantics
// as [Apply].
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
