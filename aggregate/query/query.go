package query

import (
	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/event/query"
	"github.com/modernice/goes/event/query/version"
)

// Query is a filter for Aggregates based on their name, ID, version, and
// sorting options. It is used to selectively process Aggregates that match the
// specified criteria.
type Query struct {
	names    []string
	ids      []uuid.UUID
	versions version.Constraints
	sortings []aggregate.SortOptions
}

// Option is a function that modifies a query builder. It allows to configure
// the aggregate Query by adding names, IDs, version constraints, and sorting
// options to the builder. Multiple Option functions can be combined to create
// complex queries.
type Option func(*builder)

type builder struct {
	Query

	versionConstraints []version.Option
}

// New creates a Query with the provided options. The returned Query can be used
// to filter aggregates based on their name, ID, version, and sorting options.
func New(opts ...Option) Query {
	var b builder
	return b.build(opts...)
}

// Merge combines multiple aggregate.Query instances into a single Query. It
// merges the names, IDs, and version constraints of the provided queries and
// returns a new Query with the merged options.
func Merge(queries ...aggregate.Query) Query {
	var opts []Option
	versionConstraints := make([]version.Constraints, 0, len(queries))
	for _, q := range queries {
		opts = append(opts, Name(q.Names()...), ID(q.IDs()...))
		versionConstraints = append(versionConstraints, q.Versions())
	}
	return New(append(opts, Version(version.DryMerge(versionConstraints...)...))...)
}

// Name adds the specified names to the aggregate names filter in the query
// builder.
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

// ID adds the specified UUIDs to the query, ensuring that only unique IDs are
// stored. The query will match aggregates with any of the provided IDs.
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

// Version adds the specified version constraints to the query, ensuring that
// only Aggregates with matching versions are included. The constraints are
// combined using the version package's Filter function.
func Version(constraints ...version.Option) Option {
	return func(b *builder) {
		b.versionConstraints = append(b.versionConstraints, constraints...)
	}
}

// SortBy sets the sorting option for the aggregate query by specifying the sort
// field and direction. It replaces any existing sortings with the provided one.
func SortBy(sort aggregate.Sorting, dir aggregate.SortDirection) Option {
	return func(b *builder) {
		b.sortings = []aggregate.SortOptions{{Sort: sort, Dir: dir}}
	}
}

// SortByMulti appends multiple aggregate.SortOptions to the sortings of a
// Query.
func SortByMulti(sorts ...aggregate.SortOptions) Option {
	return func(b *builder) {
		b.sortings = append(b.sortings, sorts...)
	}
}

// Tagger is an interface that provides a method to check if a specific tag is
// present. It is used to filter aggregates based on their tags.
type Tagger interface {
	// HasTag checks if the specified tag is present in the Tagger interface. It
	// returns true if the tag is found, and false otherwise.
	HasTag(string) bool
}

type queryWithTags interface {
	aggregate.Query

	// Tags returns the tags associated with the queryWithTags interface. It is used
	// to filter aggregates based on their tags.
	Tags() []string
}

func Test[D any](q aggregate.Query, a aggregate.Aggregate) bool {
	id, name, v := a.Aggregate()

	if names := q.Names(); len(names) > 0 {
		if !stringsContains(names, name) {
			return false
		}
	}

	if ids := q.IDs(); len(ids) > 0 {
		if !uuidsContains(ids, id) {
			return false
		}
	}

	if versions := q.Versions(); versions != nil {
		if exact := versions.Exact(); len(exact) > 0 &&
			!intsContains(exact, v) {
			return false
		}
		if ranges := versions.Ranges(); len(ranges) > 0 &&
			!testVersionRanges(ranges, v) {
			return false
		}
		if min := versions.Min(); len(min) > 0 &&
			!testMinVersions(min, v) {
			return false
		}
		if max := versions.Max(); len(max) > 0 &&
			!testMaxVersions(max, v) {
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

// EventQueryOpts returns a slice of query.Option for an aggregate.Query,
// converting the aggregate query's names, IDs, and version constraints into
// corresponding event query options.
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

// Names returns a slice of aggregate names that the Query targets.
func (q Query) Names() []string {
	return q.names
}

// IDs returns a slice of UUIDs that the Query filters by.
func (q Query) IDs() []uuid.UUID {
	return q.ids
}

// Versions returns the version constraints of the Query, which are used to
// filter aggregates based on their versions.
func (q Query) Versions() version.Constraints {
	return q.versions
}

// Sortings returns the sorting options of the Query. The returned sort options
// determine the order in which Aggregates should be sorted when processing the
// Query.
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
