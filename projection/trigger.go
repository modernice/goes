package projection

import (
	"github.com/modernice/goes/event"
)

// TriggerOption is a Trigger option.
type TriggerOption func(*Trigger)

// A Trigger is used by Schedules to trigger a Job.
type Trigger struct {
	// Reset projections before applying events.
	Reset bool

	// Override the Query that is used to query events from the event store.
	Query event.Query

	// Additional filters that are applied in-memory to the query result from
	// the event store.
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

// Reset returns a TriggerOption that resets projections before applying events
// onto them. Resetting a projection is done by first resetting the progress of
// the projection (if it implements progressor). Then, if the projection has a
// `Reset()` method, that method is called to allow for custom reset logic.
func Reset() TriggerOption {
	return func(t *Trigger) {
		t.Reset = true
	}
}

// Query returns a TriggerOption that sets the Query of a Trigger.
//
// When a Job is created by a Schedule, it is passed an event query to fetch the
// events for the projections. By default, this query fetches all events that
// are configured in the triggered Schedule, sorted by time. A custom query may
// be provided using the `Query` option. Don't forget to configure correct
// sorting when providing a custom query:
//
//	var s projection.Schedule
//	err := s.Trigger(context.TODO(), projection.Query(query.New(
//		query.AggregateName("foo", "bar"),
//		query.SortBy(event.SortTime, event.SortAsc), // to ensure correct sorting
//	)))
func Query(q event.Query) TriggerOption {
	return func(t *Trigger) {
		t.Query = q
	}
}

// Filter returns a TriggerOption that adds filters to a Trigger.
//
// Queried events can be further filtered using the `Filter` option. Filters
// are applied in-memory, after the events have been fetched from the event
// store. When multiple filters are configured, events must match against every
// filter to be applied to projections. Sorting options of filters are ignored.
//
//	var s projection.Schedule
//	err := s.Trigger(context.TODO(), projection.Filter(query.New(...), query.New(...)))
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
