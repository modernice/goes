package projection

import (
	"github.com/modernice/goes/event"
)

// TriggerOption configures a [Trigger].
type TriggerOption func(*Trigger)

// Trigger configures how a schedule builds a job.
type Trigger struct {
	// Reset clears projections before applying events.
	Reset bool

	// Query overrides the event fetch query.
	Query event.Query

	// AggregateQuery overrides the query used to extract aggregates.
	AggregateQuery event.Query

	// Filter holds additional in-memory filters for the job's events.
	Filter []event.Query
}

// NewTrigger builds a Trigger from opts.
func NewTrigger(opts ...TriggerOption) Trigger {
	var t Trigger
	for _, opt := range opts {
		opt(&t)
	}
	return t
}

// Reset requests that projections reset before applying events.
func Reset(reset bool) TriggerOption {
	return func(t *Trigger) {
		t.Reset = reset
	}
}

// Query sets the event query used to fetch events.
func Query(q event.Query) TriggerOption {
	return func(t *Trigger) {
		t.Query = q
	}
}

// AggregateQuery sets the query used to extract aggregates for a job.
func AggregateQuery(q event.Query) TriggerOption {
	return func(t *Trigger) {
		t.AggregateQuery = q
	}
}

// Filter adds in-memory filters to the job's events.
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

// JobOptions returns JobOptions derived from t.
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
