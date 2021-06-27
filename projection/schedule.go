package projection

import "github.com/modernice/goes/event"

// TriggerOption is a Trigger option.
type TriggerOption func(*Trigger)

// A Trigger is used by Schedules to trigger a Job.
type Trigger struct {
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
