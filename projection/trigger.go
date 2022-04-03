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

	// If provided, overrides the query that is used to fetch events that are
	// applied onto projections.
	Query event.Query

	// If provided, overrides the query that is used to extract events from a
	// projection job. The `Aggregates()` and `Aggregate()` methods of a
	// projection job will use this query.
	AggregateQuery event.Query

	// Additional filters that are applied in-memory to the query result of a
	// job's `EventsFor()` and `Apply()` methods.
	Filter []event.Query
}

// NewTrigger returns a projection trigger.
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
func Reset(reset bool) TriggerOption {
	return func(t *Trigger) {
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
func Query(q event.Query) TriggerOption {
	return func(t *Trigger) {
		t.Query = q
	}
}

// AggregateQuery returns a TriggerOption that sets the AggregateQuery of a Trigger.
//
// The `Aggregates()` and `Aggregate()` methods of a projection job will use
// this query to extract the aggregates from the projection job.
//
//	var s projection.Schedule
//	err := s.Trigger(context.TODO(), projection.AggregateQuery(query.New(
//		query.Name("foo", "bar"), // extract aggregates from "foo" and "bar" events
//	)))
func AggregateQuery(q event.Query) TriggerOption {
	return func(t *Trigger) {
		t.AggregateQuery = q
	}
}

// Filter returns a TriggerOption that adds filters to a Trigger.
//
// Filters are applied in-memory, after the events have been fetched from the
// event store. When multiple filters are configured, events must match against
// every filter to be applied to projections. Sorting options of queries are
// ignored.
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
		opts = append(opts, Reset(true))
	}
	if t.Query != nil {
		opts = append(opts, Query(t.Query))
	}
	if t.AggregateQuery != nil {
		opts = append(opts, AggregateQuery(t.AggregateQuery))
	}
	if len(t.Filter) > 0 {
		opts = append(opts, Filter(t.Filter...))
	}
	return opts
}

// JobOptions returns the options for a job that is triggered by this trigger.
func (t Trigger) JobOptions() []JobOption {
	var opts []JobOption
	if t.Reset {
		opts = append(opts, WithReset())
	}
	if t.AggregateQuery != nil {
		opts = append(opts, WithAggregateQuery(t.AggregateQuery))
	}
	if len(t.Filter) > 0 {
		opts = append(opts, WithFilter(t.Filter...))
	}
	return opts
}
