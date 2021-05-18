package query

import (
	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/event/query"
	"github.com/modernice/goes/event/query/version"
)

// Query is used by aggregate repositories to filter aggregates.
type Query struct {
	names    []string
	ids      []uuid.UUID
	tags     []string
	versions version.Constraints
	sortings []aggregate.SortOptions
}

// Option is a query option.
type Option func(*builder)

type builder struct {
	Query

	versionConstraints []version.Option
}

// New returns a Query that is built from opts.
func New(opts ...Option) Query {
	var b builder
	return b.build(opts...)
}

// Merge merges multiple Queries into one.
func Merge(queries ...aggregate.Query) Query {
	var opts []Option
	versionConstraints := make([]version.Constraints, 0, len(queries))
	for _, q := range queries {
		opts = append(opts, Name(q.Names()...), ID(q.IDs()...))
		versionConstraints = append(versionConstraints, q.Versions())
	}
	return New(append(opts, Version(version.DryMerge(versionConstraints...)...))...)
}

// Name returns an Option that filters aggregates by their names.
func Name(names ...string) Option {
	return func(b *builder) {
	L:
		for _, name := range names {
			for _, name2 := range b.names {
				if name == name2 {
					continue L
				}
			}
			b.names = append(b.names, name)
		}
	}
}

// ID returns an Option that filters aggregates by their ids.
func ID(ids ...uuid.UUID) Option {
	return func(b *builder) {
	L:
		for _, id := range ids {
			for _, id2 := range b.ids {
				if id == id2 {
					continue L
				}
			}
			b.ids = append(b.ids, id)
		}
	}
}

// Tag returns an Option that filters aggregates by their tags.
func Tag(tags ...string) Option {
	return func(b *builder) {
	L:
		for _, tag := range tags {
			for _, tag2 := range b.tags {
				if tag == tag2 {
					continue L
				}
			}
			b.tags = append(b.tags, tag)
		}
	}
}

// Version returns an Option that filters aggregates by their versions.
func Version(constraints ...version.Option) Option {
	return func(b *builder) {
		b.versionConstraints = append(b.versionConstraints, constraints...)
	}
}

// SortBy returns an Option that defines the sorting behaviour for a Query.
func SortBy(sort aggregate.Sorting, dir aggregate.SortDirection) Option {
	return func(b *builder) {
		b.sortings = []aggregate.SortOptions{{Sort: sort, Dir: dir}}
	}
}

// SortByMulti returns an Option that defines the sorting behaviour for a Query.
func SortByMulti(sorts ...aggregate.SortOptions) Option {
	return func(b *builder) {
		b.sortings = append(b.sortings, sorts...)
	}
}

// Tagger is an aggregate that implements tagging.
//
// Tagger is implemented by embedding *tagging.Tagger into an aggregate.
type Tagger interface {
	HasTag(string) bool
}

type queryWithTags interface {
	aggregate.Query

	Tags() []string
}

// Test tests the Aggregate a against the Query q and returns true if q should
// include a in its results. Test can be used by in-memory aggregate.Repository
// implementations to filter aggregates based on the query.
func Test(q aggregate.Query, a aggregate.Aggregate) bool {
	if names := q.Names(); len(names) > 0 {
		if !stringsContains(names, a.AggregateName()) {
			return false
		}
	}

	if ids := q.IDs(); len(ids) > 0 {
		if !uuidsContains(ids, a.AggregateID()) {
			return false
		}
	}

	if versions := q.Versions(); versions != nil {
		if exact := versions.Exact(); len(exact) > 0 &&
			!intsContains(exact, a.AggregateVersion()) {
			return false
		}
		if ranges := versions.Ranges(); len(ranges) > 0 &&
			!testVersionRanges(ranges, a.AggregateVersion()) {
			return false
		}
		if min := versions.Min(); len(min) > 0 &&
			!testMinVersions(min, a.AggregateVersion()) {
			return false
		}
		if max := versions.Max(); len(max) > 0 &&
			!testMaxVersions(max, a.AggregateVersion()) {
			return false
		}
	}

	if tagger, ok := a.(Tagger); ok {
		if q, ok := q.(queryWithTags); ok {
			if tags := q.Tags(); len(tags) > 0 {
				var hasTag bool
				for _, tag := range tags {
					if tagger.HasTag(tag) {
						hasTag = true
						break
					}
				}
				if !hasTag {
					return false
				}
			}
		}
	}

	return true
}

// EventQueryOpts returns query.Options for a given aggregate.Query.
//
// In order for the returned Query to return the correct Events, EventQueryOpts
// needs to rewrite some of the version filters to make sense for an Aggregate-
// specific event.Query:
// 	- version.Exact is rewritten to version.Max
// 		(querying for version 10 of an Aggregate should return Events 1 -> 10)
// 	- version.Max is passed without modification
// 	- version.Min is discarded
// 		(because an Aggregate cannot start at a version > 1)
// 	- version.Ranges is rewritten to version.Max
func EventQueryOpts(q aggregate.Query) []query.Option {
	var opts []query.Option
	if names := q.Names(); len(names) > 0 {
		opts = append(opts, query.AggregateName(names...))
	}
	if ids := q.IDs(); len(ids) > 0 {
		opts = append(opts, query.AggregateID(ids...))
	}
	if versions := q.Versions(); versions != nil {
		var constraints []version.Option
		if exact := versions.Exact(); len(exact) > 0 {
			constraints = append(constraints, version.Max(exact...))
		}
		if ranges := versions.Ranges(); len(ranges) > 0 {
			max := make([]int, len(ranges))
			for i, r := range ranges {
				max[i] = r.End()
			}
			constraints = append(constraints, version.Max(max...))
		}
		if max := versions.Max(); len(max) > 0 {
			constraints = append(constraints, version.Max(max...))
		}
		opts = append(opts, query.AggregateVersion(constraints...))
	}
	return opts
}

// Names returns the aggregate names to query for.
func (q Query) Names() []string {
	return q.names
}

// IDs returns the aggregate ids to query for.
func (q Query) IDs() []uuid.UUID {
	return q.ids
}

// Tags returns the aggregate tags to query for.
func (q Query) Tags() []string {
	return q.tags
}

// Versions returns the aggregate version constraints for the query.
func (q Query) Versions() version.Constraints {
	return q.versions
}

// Sortings returns the SortConfig for the query.
func (q Query) Sortings() []aggregate.SortOptions {
	return q.sortings
}

func (b builder) build(opts ...Option) Query {
	for _, opt := range opts {
		opt(&b)
	}
	b.versions = version.Filter(b.versionConstraints...)
	return b.Query
}

func stringsContains(vals []string, s string) bool {
	for _, v := range vals {
		if v == s {
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

func intsContains(ints []int, i int) bool {
	for _, v := range ints {
		if v == i {
			return true
		}
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
