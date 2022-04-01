// Package query provides an event query builder.
package query

import (
	"github.com/google/uuid"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/query/time"
	"github.com/modernice/goes/event/query/version"
)

// Query is used by event stores to filter events.
type Query struct {
	names          []string
	ids            []uuid.UUID
	aggregateNames []string
	aggregateIDs   []uuid.UUID
	aggregates     []event.AggregateRef
	sortings       []event.SortOptions

	times             time.Constraints
	aggregateVersions version.Constraints
}

// Option is a Query option.
type Option func(*builder)

type builder struct {
	Query
	timeConstraints    []time.Option
	versionConstraints []version.Option
}

// New builds a Query from opts.
func New(opts ...Option) Query {
	var b builder
	for _, opt := range opts {
		opt(&b)
	}
	return b.build()
}

// Name returns an Option that filters events by their names.
func Name(names ...string) Option {
	return func(b *builder) {
	L:
		for _, name := range names {
			for _, n := range b.names {
				if n == name {
					continue L
				}
			}
			b.names = append(b.names, names...)
		}
	}
}

// ID returns an Option that filters events by their ids.
func ID(ids ...uuid.UUID) Option {
	return func(b *builder) {
	L:
		for _, id := range ids {
			for _, id2 := range b.ids {
				if id2 == id {
					continue L
				}
			}
			b.ids = append(b.ids, ids...)
		}
	}
}

// Time returns an Option that filters events by time constraints.
func Time(constraints ...time.Option) Option {
	return func(b *builder) {
		b.timeConstraints = append(b.timeConstraints, constraints...)
	}
}

// AggregateName returns an Option that filters events by their aggregate names.
func AggregateName(names ...string) Option {
	return func(b *builder) {
	L:
		for _, name := range names {
			for _, n := range b.aggregateNames {
				if n == name {
					continue L
				}
			}
			b.aggregateNames = append(b.aggregateNames, name)
		}
	}
}

// AggregateID returns an Option that filters events by their aggregate ids.
func AggregateID(ids ...uuid.UUID) Option {
	return func(b *builder) {
	L:
		for _, id := range ids {
			for _, id2 := range b.aggregateIDs {
				if id2 == id {
					continue L
				}
			}
			b.aggregateIDs = append(b.aggregateIDs, id)
		}
	}
}

// AggregateVersion returns an Option that filters events by their aggregate
// versions.
func AggregateVersion(constraints ...version.Option) Option {
	return func(b *builder) {
		b.versionConstraints = append(b.versionConstraints, constraints...)
	}
}

// Aggregate returns an Option that filters events by a specific aggregate.
func Aggregate(name string, id uuid.UUID) Option {
	return func(b *builder) {
		for _, at := range b.aggregates {
			if at.Name == name && at.ID == id {
				return
			}
		}
		b.aggregates = append(b.aggregates, event.AggregateRef{Name: name, ID: id})
	}
}

// Aggregates returns an Option that filters events by specific aggregates.
func Aggregates(aggregates ...event.AggregateRef) Option {
	return func(b *builder) {
	L:
		for _, at := range aggregates {
			for _, at2 := range b.aggregates {
				if at2 == at {
					continue L
				}
			}
			b.aggregates = append(b.aggregates, at)
		}
	}
}

// SortBy returns an Option that sorts a Query by the given Sorting and
// SortDirection.
func SortBy(sort event.Sorting, dir event.SortDirection) Option {
	return SortByMulti(event.SortOptions{Sort: sort, Dir: dir})
}

// SortByMulti return an Option that sorts a Query by multiple Sortings and
// SortDirections.
func SortByMulti(sorts ...event.SortOptions) Option {
	return func(b *builder) {
	L:
		for _, s := range sorts {
			for _, s2 := range b.sortings {
				if s2 == s {
					continue L
				}
			}
			b.sortings = append(b.sortings, s)
		}
	}
}

// SortByAggregate returns an Option that sorts the a Query by aggregates.
//
// Order of sortings is:
//	1. aggregateName (ascending)
//	2. aggregateID (ascending)
//	3. aggregateVersion (ascending)
func SortByAggregate() Option {
	return SortByMulti(
		event.SortOptions{Sort: event.SortAggregateName, Dir: event.SortAsc},
		event.SortOptions{Sort: event.SortAggregateID, Dir: event.SortAsc},
		event.SortOptions{Sort: event.SortAggregateVersion, Dir: event.SortAsc},
	)
}

// SortByTime returns an Option that sorts a Query by event time.
func SortByTime() Option {
	return SortBy(event.SortTime, event.SortAsc)
}

// Test tests the event evt against the Query q and returns true if q should
// include evt in its results. Test can be used by in-memory event.Store
// implementations to filter events based on the query.
func Test[D any](q event.Query, evt event.Of[D]) bool {
	return event.Test(q, evt)
}

// Apply tests events against the provided Query and returns only those events
// that match the Query.
func Apply[D any](q event.Query, events ...event.Of[D]) []event.Of[D] {
	if events == nil {
		return nil
	}
	out := make([]event.Of[D], 0, len(events))
	for _, evt := range events {
		if Test(q, evt) {
			out = append(out, evt)
		}
	}
	return out
}

// Merge merges multiple Queries and returns the merged Query.
//
// In cases where only a single value can be assigned to a filter, the last
// provided Query that provides that filter is used.
func Merge(queries ...event.Query) Query {
	var opts []Option
	for _, q := range queries {
		if q == nil {
			continue
		}

		versionOpts := version.DryMerge(q.AggregateVersions())
		timeOpts := time.DryMerge(q.Times())

		opts = append(
			opts,
			ID(q.IDs()...),
			Name(q.Names()...),
			AggregateID(q.AggregateIDs()...),
			AggregateName(q.AggregateNames()...),
			AggregateVersion(versionOpts...),
			Aggregates(q.Aggregates()...),
			Time(timeOpts...),
			SortByMulti(q.Sortings()...),
		)
	}
	return New(opts...)
}

// Names returns the event names to query for.
func (q Query) Names() []string {
	return q.names
}

// IDs returns the event ids to query for.
func (q Query) IDs() []uuid.UUID {
	return q.ids
}

// Times returns the time constraints. Times guarantees to return non-nil
// time.Constraints.
func (q Query) Times() time.Constraints {
	return q.times
}

// AggregateNames returns the aggregate names to query for.
func (q Query) AggregateNames() []string {
	return q.aggregateNames
}

// AggregateIDs returns the aggregate ids to query for.
func (q Query) AggregateIDs() []uuid.UUID {
	return q.aggregateIDs
}

// AggregateVersions returns the aggregate versions to query for.
func (q Query) AggregateVersions() version.Constraints {
	return q.aggregateVersions
}

// Aggregates returns a slice of specific aggregates to query for.
func (q Query) Aggregates() []event.AggregateRef {
	return q.aggregates
}

// Sortings returns the SortConfigs for the query.
func (q Query) Sortings() []event.SortOptions {
	return q.sortings
}

func (b builder) build() Query {
	b.times = time.Filter(b.timeConstraints...)
	b.aggregateVersions = version.Filter(b.versionConstraints...)
	return b.Query
}
