// Package query provides an event query builder.
package query

import (
	"github.com/google/uuid"
	"github.com/modernice/goes/event/time"
	"github.com/modernice/goes/event/version"
)

// Query is a query that's used event stores to filter events.
type Query struct {
	names          []string
	ids            []uuid.UUID
	aggregateNames []string
	aggregateIDs   []uuid.UUID

	times             time.Constraints
	aggregateVersions version.Constraints
}

// Option is a Query option.
type Option func(*builder)

type builder struct {
	Query
	timeConstraints    []time.Constraint
	versionConstraints []version.Constraint
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
		b.names = append(b.names, names...)
	}
}

// ID returns an Option that filters events by their ids.
func ID(ids ...uuid.UUID) Option {
	return func(b *builder) {
		b.ids = append(b.ids, ids...)
	}
}

// Time returns an Option that filters events by time constraints.
func Time(constraints ...time.Constraint) Option {
	return func(b *builder) {
		b.timeConstraints = append(b.timeConstraints, constraints...)
	}
}

// AggregateName returns an Option that filters events by their aggregate names.
func AggregateName(names ...string) Option {
	return func(b *builder) {
		b.aggregateNames = append(b.aggregateNames, names...)
	}
}

// AggregateID returns an Option that filters events by their aggregate ids.
func AggregateID(ids ...uuid.UUID) Option {
	return func(b *builder) {
		b.aggregateIDs = append(b.aggregateIDs, ids...)
	}
}

// AggregateVersion returns an Option that filters events by their aggregate
// versions.
func AggregateVersion(constraints ...version.Constraint) Option {
	return func(b *builder) {
		b.versionConstraints = append(b.versionConstraints, constraints...)
	}
}

// Names returns the "event name" filter.
func (q Query) Names() []string {
	return q.names
}

// IDs returns the "event id" filter.
func (q Query) IDs() []uuid.UUID {
	return q.ids
}

// Times returns the time constraints. Times guarantees to return non-nil
// time.Constraints.
func (q Query) Times() time.Constraints {
	return q.times
}

// AggregateNames returns the "aggregate name" filter.
func (q Query) AggregateNames() []string {
	return q.aggregateNames
}

// AggregateIDs returns the "aggregate id" filter.
func (q Query) AggregateIDs() []uuid.UUID {
	return q.aggregateIDs
}

// AggregateVersions returns the "aggregate version" filter.
func (q Query) AggregateVersions() version.Constraints {
	return q.aggregateVersions
}

func (b builder) build() Query {
	b.times = time.Filter(b.timeConstraints...)
	b.aggregateVersions = version.Filter(b.versionConstraints...)
	return b.Query
}
