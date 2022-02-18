package query

import (
	stdtime "time"

	"github.com/modernice/goes"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/query"
	"github.com/modernice/goes/aggregate/snapshot"
	"github.com/modernice/goes/event/query/time"
	"github.com/modernice/goes/event/query/version"
)

// Query is used by Snapshot Stores to filter Snapshots.
type Query[ID goes.ID] struct {
	query.Query[ID]

	times time.Constraints
}

type Option func(*builder)

type builder struct {
	Query[goes.AID]

	opts            []query.Option
	timeConstraints []time.Option
}

// Name returns an Option that filters Snapshots by their AggregateName.
func Name(names ...string) Option {
	return func(b *builder) {
		b.opts = append(b.opts, query.Name(names...))
	}
}

// ID returns an Option that filters Snapshots by their AggregateID:
func ID[ID goes.ID](ids ...ID) Option {
	return func(b *builder) {
		b.opts = append(b.opts, query.ID(ids...))
	}
}

// Version returns an Option that filters Snapshots by their AggregateVersion.
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
func New[ID goes.ID](opts ...Option) Query[ID] {
	return buildQuery[ID](builder{}, opts...)
}

// Test tests the Snapshot s against the Query q and returns true if q should
// include s in its results. Test can be used by snapshot.Store implementations
// to filter events based on the query.
func Test[ID goes.ID](q snapshot.Query[ID], s snapshot.Snapshot[ID]) bool {
	if !query.Test[any, ID](q, aggregate.New(
		s.AggregateName(),
		s.AggregateID(),
		aggregate.Version(s.AggregateVersion()),
	)) {
		return false
	}

	if times := q.Times(); times != nil {
		if exact := times.Exact(); len(exact) > 0 &&
			!timesContains(exact, s.Time()) {
			return false
		}
		if ranges := times.Ranges(); len(ranges) > 0 &&
			!testTimeRanges(ranges, s.Time()) {
			return false
		}
		if min := times.Min(); !min.IsZero() && !testMinTimes(min, s.Time()) {
			return false
		}
		if max := times.Max(); !max.IsZero() && !testMaxTimes(max, s.Time()) {
			return false
		}
	}

	return true
}

// Times returns the time.Constraints of the Query.
func (q Query[ID]) Times() time.Constraints {
	return q.times
}

func buildQuery[ID goes.ID](b builder, opts ...Option) Query[ID] {
	for _, opt := range opts {
		opt(&b)
	}

	return Query[ID]{
		Query: query.New[ID](b.opts...),
		times: time.Filter(b.timeConstraints...),
	}
}

func timesContains(times []stdtime.Time, t stdtime.Time) bool {
	for _, v := range times {
		if v.Equal(t) {
			return true
		}
	}
	return false
}

func testTimeRanges(ranges []time.Range, t stdtime.Time) bool {
	for _, r := range ranges {
		if r.Includes(t) {
			return true
		}
	}
	return false
}

func testMinTimes(min stdtime.Time, t stdtime.Time) bool {
	if t.Equal(min) || t.After(min) {
		return true
	}
	return false
}

func testMaxTimes(max stdtime.Time, t stdtime.Time) bool {
	if t.Equal(max) || t.Before(max) {
		return true
	}
	return false
}
