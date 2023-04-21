package query

import (
	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/query"
	"github.com/modernice/goes/event/query/time"
	"github.com/modernice/goes/event/query/version"
)

// Query is used by Snapshot Stores to filter Snapshots.
type Query struct {
	query.Query

	times time.Constraints
}

// Option is a type that defines a function signature for modifying a builder.
// It is used to apply filtering options to Snapshot Stores in the query
// package.
type Option func(*builder)

type builder struct {
	Query

	opts            []query.Option
	timeConstraints []time.Option
}

// Name returns an Option that filters Snapshots by their aggregateName.
func Name(names ...string) Option {
	return func(b *builder) {
		b.opts = append(b.opts, query.Name(names...))
	}
}

// ID returns an Option that filters Snapshots by their aggregateID:
func ID(ids ...uuid.UUID) Option {
	return func(b *builder) {
		b.opts = append(b.opts, query.ID(ids...))
	}
}

// Version returns an Option that filters Snapshots by their aggregateVersion.
func Version(constraints ...version.Option) Option {
	return func(b *builder) {
		b.opts = append(b.opts, query.Version(constraints...))
	}
}

// Time returns an Option that filters Snapshots by the time they were created.
func Time(constraints ...time.Option) Option {
	return func(b *builder) {
		b.timeConstraints = append(b.timeConstraints, constraints...)
	}
}

// SortBy returns an Option that defines the sorting behaviour for a Query.
func SortBy(sort aggregate.Sorting, dir aggregate.SortDirection) Option {
	return func(b *builder) {
		b.opts = append(b.opts, query.SortBy(sort, dir))
	}
}

// SortByMulti returns an Option that defines the sorting behaviour for a Query.
func SortByMulti(sorts ...aggregate.SortOptions) Option {
	return func(b *builder) {
		b.opts = append(b.opts, query.SortByMulti(sorts...))
	}
}

// New returns a Query from opts.
func New(opts ...Option) Query {
	var b builder
	return b.build(opts...)
}

// Times returns the time.Constraints of the Query.
func (q Query) Times() time.Constraints {
	return q.times
}

func (b *builder) build(opts ...Option) Query {
	for _, opt := range opts {
		opt(b)
	}
	b.Query.Query = query.New(b.opts...)
	b.times = time.Filter(b.timeConstraints...)
	return b.Query
}
