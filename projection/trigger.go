package projection

import (
	"github.com/modernice/goes"
	"github.com/modernice/goes/event"
)

// TriggerOption is a Trigger option.
type TriggerOption[ID goes.ID] func(*Trigger[ID])

// A Trigger is used by Schedules to trigger a Job.
type Trigger[ID goes.ID] struct {
	// Reset projections before applying events.
	Reset bool

	// Override the Query that is used to query events from the event store.
	Query event.QueryOf[ID]

	// Additional filters that are applied in-memory to the query result from
	// the event store.
	Filter []event.QueryOf[ID]
}

// NewTrigger returns a Trigger.
func NewTrigger[ID goes.ID](opts ...TriggerOption[ID]) Trigger[ID] {
	var t Trigger[ID]
	for _, opt := range opts {
		opt(&t)
	}
	return t
}

// Reset returns a TriggerOption that resets projections before applying events
// onto them. Resetting a projection is done by first resetting the progress of
// the projection (if it implements progressor). Then, if the projection has a
// `Reset()` method, that method is called to allow for custom reset logic.
func Reset[ID goes.ID](reset bool) TriggerOption[ID] {
	return func(t *Trigger[ID]) {
		t.Reset = reset
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
func Query[ID goes.ID](q event.QueryOf[ID]) TriggerOption[ID] {
	return func(t *Trigger[ID]) {
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
func Filter[ID goes.ID](queries ...event.QueryOf[ID]) TriggerOption[ID] {
	return func(t *Trigger[ID]) {
		t.Filter = append(t.Filter, queries...)
	}
}

// Options returns the TriggerOptions to build t.
func (t Trigger[ID]) Options() []TriggerOption[ID] {
	var opts []TriggerOption[ID]
	if t.Reset {
		opts = append(opts, Reset[ID](true))
	}
	if t.Query != nil {
		opts = append(opts, Query[ID](t.Query))
	}
	if len(t.Filter) > 0 {
		opts = append(opts, Filter[ID](t.Filter...))
	}
	return opts
}
