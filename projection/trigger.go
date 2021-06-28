package projection

import (
	"github.com/modernice/goes/event"
)

// TriggerOption is a Trigger option.
type TriggerOption func(*Trigger)

// A Trigger is used by Schedules to trigger a Job.
type Trigger struct {
	Reset  bool
	Query  event.Query
	Filter []event.Query
}

// NewTrigger returns a Trigger.
func NewTrigger(opts ...TriggerOption) Trigger {
	var t Trigger
	for _, opt := range opts {
		opt(&t)
	}
	return t
}

// Reset returns a TriggerOption that resets Projections before applying Events
// onto them. Resetting a Projection is done by first resetting the progress of
// the Projection (if it implements progressor). Then, if the Projection has a
// Reset method, that method is called to allow for custom reset logic.
func Reset() TriggerOption {
	return func(t *Trigger) {
		t.Reset = true
	}
}

// Query returns a TriggerOption that sets the Query of the Trigger.
func Query(q event.Query) TriggerOption {
	return func(t *Trigger) {
		t.Query = q
	}
}

// Filter returns a TriggerOption that adds filters to the Trigger.
func Filter(queries ...event.Query) TriggerOption {
	return func(t *Trigger) {
		t.Filter = append(t.Filter, queries...)
	}
}

// Options returns the TriggerOptions to build t.
func (t Trigger) Options() []TriggerOption {
	var opts []TriggerOption
	if t.Reset {
		opts = append(opts, Reset())
	}
	if t.Query != nil {
		opts = append(opts, Query(t.Query))
	}
	if len(t.Filter) > 0 {
		opts = append(opts, Filter(t.Filter...))
	}
	return opts
}
