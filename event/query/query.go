// Package query provides an event query builder.
package query

import (
	stdtime "time"

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
	sortings       []event.SortOptions

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

// SortBy returns an Option that defines the sorting behaviour for a query.
func SortBy(sort event.Sorting, dir event.SortDirection) Option {
	return func(b *builder) {
		b.sortings = []event.SortOptions{{Sort: sort, Dir: dir}}
	}
}

// SortByMulti returns an Option that defines the sorting behaviour for a query.
func SortByMulti(sorts ...event.SortOptions) Option {
	return func(b *builder) {
		b.sortings = append(b.sortings, sorts...)
	}
}

// Test tests the Event evt against the Query q and returns true if q should
// include evt in its results. Test can be used by in-memory event.Store
// implementations to filter events based on the query.
func Test(q event.Query, evt event.Event) bool {
	if q == nil {
		return true
	}

	if names := q.Names(); len(names) > 0 &&
		!stringsContains(names, evt.Name()) {
		return false
	}

	if ids := q.IDs(); len(ids) > 0 && !uuidsContains(ids, evt.ID()) {
		return false
	}

	if times := q.Times(); times != nil {
		if exact := times.Exact(); len(exact) > 0 &&
			!timesContains(exact, evt.Time()) {
			return false
		}
		if ranges := times.Ranges(); len(ranges) > 0 &&
			!testTimeRanges(ranges, evt.Time()) {
			return false
		}
		if min := times.Min(); !min.IsZero() && !testMinTimes(min, evt.Time()) {
			return false
		}
		if max := times.Max(); !max.IsZero() && !testMaxTimes(max, evt.Time()) {
			return false
		}
	}

	if names := q.AggregateNames(); len(names) > 0 &&
		!stringsContains(names, evt.AggregateName()) {
		return false
	}

	if ids := q.AggregateIDs(); len(ids) > 0 &&
		!uuidsContains(ids, evt.AggregateID()) {
		return false
	}

	if versions := q.AggregateVersions(); versions != nil {
		if exact := versions.Exact(); len(exact) > 0 &&
			!intsContains(exact, evt.AggregateVersion()) {
			return false
		}
		if ranges := versions.Ranges(); len(ranges) > 0 &&
			!testVersionRanges(ranges, evt.AggregateVersion()) {
			return false
		}
		if min := versions.Min(); len(min) > 0 &&
			!testMinVersions(min, evt.AggregateVersion()) {
			return false
		}
		if max := versions.Max(); len(max) > 0 &&
			!testMaxVersions(max, evt.AggregateVersion()) {
			return false
		}
	}

	return true
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

		var versionOpts []version.Constraint

		if versions := q.AggregateVersions(); versions != nil {
			versionOpts = []version.Constraint{
				version.Exact(versions.Exact()...),
				version.InRange(versions.Ranges()...),
				version.Min(versions.Min()...),
				version.Max(versions.Max()...),
			}
		}

		var timeOpts []time.Constraint

		if times := q.Times(); times != nil {
			timeOpts = []time.Constraint{
				time.Exact(q.Times().Exact()...),
				time.InRange(q.Times().Ranges()...),
			}

			if min := times.Min(); !min.IsZero() {
				timeOpts = append(timeOpts, time.Min(min))
			}

			if max := times.Max(); !max.IsZero() {
				timeOpts = append(timeOpts, time.Max(max))
			}
		}

		opts = append(
			opts,
			ID(q.IDs()...),
			Name(q.Names()...),
			AggregateID(q.AggregateIDs()...),
			AggregateName(q.AggregateNames()...),
			AggregateVersion(versionOpts...),
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

// Sortings returns the SortConfigs for the query.
func (q Query) Sortings() []event.SortOptions {
	return q.sortings
}

func (b builder) build() Query {
	b.times = time.Filter(b.timeConstraints...)
	b.aggregateVersions = version.Filter(b.versionConstraints...)
	return b.Query
}

func stringsContains(vals []string, val string) bool {
	for _, v := range vals {
		if v == val {
			return true
		}
	}
	return false
}

func uuidsContains(ids []uuid.UUID, id uuid.UUID) bool {
	for _, i := range ids {
		if i == id {
			return true
		}
	}
	return false
}

func timesContains(times []stdtime.Time, t stdtime.Time) bool {
	for _, v := range times {
		if v.Equal(t) {
			return true
		}
	}
	return false
}

func intsContains(ints []int, i int) bool {
	for _, v := range ints {
		if v == i {
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

func testMinTimes(max stdtime.Time, t stdtime.Time) bool {
	if t.Equal(max) || t.After(max) {
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

func testVersionRanges(ranges []version.Range, v int) bool {
	for _, r := range ranges {
		if r.Includes(v) {
			return true
		}
	}
	return false
}

func testMinVersions(min []int, v int) bool {
	for _, m := range min {
		if v >= m {
			return true
		}
	}
	return false
}

func testMaxVersions(max []int, v int) bool {
	for _, m := range max {
		if v <= m {
			return true
		}
	}
	return false
}
