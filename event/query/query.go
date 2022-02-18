// Package query provides an event query builder.
package query

import (
	"github.com/modernice/goes"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/query/time"
	"github.com/modernice/goes/event/query/version"
)

var _ event.QueryOf[goes.SID] = Query[goes.SID]{}

// Query is used by event stores to filter events.
type Query[ID goes.ID] struct {
	names          []string
	ids            []ID
	aggregateNames []string
	aggregateIDs   []ID
	aggregates     []event.AggregateRefOf[ID]
	sortings       []event.SortOptions

	times             time.Constraints
	aggregateVersions version.Constraints
}

// Option is a Query option.
type Option func(*builder)

type builder struct {
	Query[goes.AID]

	timeConstraints    []time.Option
	versionConstraints []version.Option
}

// New builds a Query from opts.
func New[ID goes.ID](opts ...Option) Query[ID] {
	var b builder
	for _, opt := range opts {
		opt(&b)
	}
	return buildQuery[ID](b)
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
func ID[ID goes.ID](ids ...ID) Option {
	sids := make([]goes.AID, len(ids))
	for i, id := range ids {
		sids[i] = goes.AnyID(id)
	}

	return func(b *builder) {
	L:
		for _, id := range sids {
			for _, id2 := range b.ids {
				if id2.String() == id.String() {
					continue L
				}
			}
			b.ids = append(b.ids, id)
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
func AggregateID[ID goes.ID](ids ...ID) Option {
	sids := make([]goes.AID, len(ids))
	for i, id := range ids {
		sids[i] = goes.AnyID(id)
	}

	return func(b *builder) {
	L:
		for _, id := range sids {
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

// Aggregate returns an Option that filters Events by a specific Aggregate.
func Aggregate[ID goes.ID](name string, id ID) Option {
	return func(b *builder) {
		for _, at := range b.aggregates {
			if at.Name == name && at.ID.String() == id.String() {
				return
			}
		}
		b.aggregates = append(b.aggregates, event.AggregateRefOf[goes.AID]{Name: name, ID: goes.AnyID(id)})
	}
}

// Aggregates returns an Option that filters Events by specific Aggregates.
func Aggregates[ID goes.ID](aggregates ...event.AggregateRefOf[ID]) Option {
	return func(b *builder) {
	L:
		for _, at := range aggregates {
			for _, at2 := range b.aggregates {
				if at2.Name == at.Name && at2.ID.String() == at.ID.String() {
					continue L
				}
			}
			b.aggregates = append(b.aggregates, event.AggregateRefOf[goes.AID]{
				Name: at.Name,
				ID:   goes.AnyID(at.ID),
			})
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
//	1. AggregateName (ascending)
//	2. AggregateID (ascending)
//	3. AggregateVersion (ascending)
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

// Test tests the Event evt against the Query q and returns true if q should
// include evt in its results. Test can be used by in-memory event.Store
// implementations to filter events based on the query.
func Test[D any, ID goes.ID](q event.QueryOf[ID], evt event.Of[D, ID]) bool {
	return event.Test(q, evt)
}

// Apply tests events against the provided Query and returns only those events
// that match the Query.
func Apply[D any, ID goes.ID](q event.QueryOf[ID], events ...event.Of[D, ID]) []event.Of[D, ID] {
	if events == nil {
		return nil
	}
	out := make([]event.Of[D, ID], 0, len(events))
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
func Merge[Data any, IDType goes.ID, Queries ~[]event.QueryOf[IDType]](queries Queries) Query[IDType] {
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
	return New[IDType](opts...)
}

// Names returns the event names to query for.
func (q Query[ID]) Names() []string {
	return q.names
}

// IDs returns the event ids to query for.
func (q Query[ID]) IDs() []ID {
	return q.ids
}

// Times returns the time constraints. Times guarantees to return non-nil
// time.Constraints.
func (q Query[ID]) Times() time.Constraints {
	return q.times
}

// AggregateNames returns the aggregate names to query for.
func (q Query[ID]) AggregateNames() []string {
	return q.aggregateNames
}

// AggregateIDs returns the aggregate ids to query for.
func (q Query[ID]) AggregateIDs() []ID {
	return q.aggregateIDs
}

// AggregateVersions returns the aggregate versions to query for.
func (q Query[ID]) AggregateVersions() version.Constraints {
	return q.aggregateVersions
}

// Aggregates returns a slice of specific Aggregates to query for.
func (q Query[ID]) Aggregates() []event.AggregateRefOf[ID] {
	return q.aggregates
}

// Sortings returns the SortConfigs for the query.
func (q Query[ID]) Sortings() []event.SortOptions {
	return q.sortings
}

func buildQuery[ID goes.ID](b builder) Query[ID] {
	ids := make([]ID, len(b.Query.ids))
	for i, id := range b.Query.ids {
		ids[i] = any(id).(goes.AID).ID.(ID)
	}

	aggregateIDs := make([]ID, len(b.Query.aggregateIDs))
	for i, id := range b.Query.aggregateIDs {
		aggregateIDs[i] = any(id).(goes.AID).ID.(ID)
	}

	aggregates := make([]event.AggregateRefOf[ID], len(b.Query.aggregates))
	for i, a := range b.Query.aggregates {
		aggregates[i] = event.AggregateRefOf[ID]{
			Name: a.Name,
			ID:   any(a.ID).(goes.AID).ID.(ID),
		}
	}

	return Query[ID]{
		names:             b.Query.names,
		ids:               ids,
		aggregateNames:    b.Query.aggregateNames,
		aggregateIDs:      aggregateIDs,
		aggregates:        aggregates,
		sortings:          b.Query.sortings,
		times:             time.Filter(b.timeConstraints...),
		aggregateVersions: version.Filter(b.versionConstraints...),
	}
}
