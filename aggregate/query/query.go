package query

import (
	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/event/query"
	"github.com/modernice/goes/event/query/version"
)

// Query filters aggregates by names, ids, versions and sort order. It can be
// built with Options and merged or expanded from other queries.
type Query struct {
	Q

	versionConstraints []version.Option
}

// Q holds the actual filter fields used by Query.
type Q struct {
	Names    []string
	IDs      []uuid.UUID
	Versions version.Constraints
	Sortings []aggregate.SortOptions
}

// Option modifies a [Query].
type Option func(*Query)

// New constructs a new [Query] with the provided options. The options can
// specify names, IDs, version constraints, and sorting options to fine-tune the
// behavior of the [Query]. If no options are provided, it returns an empty
// [Query].
func New(opts ...Option) Query {
	var q Query
	for _, opt := range opts {
		opt(&q)
	}
	q.Q.Versions = version.Filter(q.versionConstraints...)
	q.versionConstraints = nil
	return q
}

// Expand converts an aggregate.Query into a Query. If the provided
// aggregate.Query is already a Query, it is returned unchanged. Otherwise, a
// new Query is created with the names, IDs, version constraints and sorting
// options of the provided aggregate.Query.
func Expand(q aggregate.Query) Query {
	if q, ok := q.(Query); ok {
		return q
	}

	v := q.Versions()

	return New(
		Name(q.Names()...),
		ID(q.IDs()...),
		Version(
			version.Exact(v.Exact()...),
			version.Min(v.Min()...),
			version.Max(v.Max()...),
			version.InRange(v.Ranges()...),
		),
		SortByMulti(q.Sortings()...),
	)
}

// Merge combines multiple aggregate queries into a single query. The resulting
// query includes the names, IDs, and version constraints of each provided
// query. The returned query can be used to filter aggregates that match any of
// the criteria specified in the merged queries.
func Merge(queries ...aggregate.Query) Query {
	var opts []Option
	versionConstraints := make([]version.Constraints, 0, len(queries))
	for _, q := range queries {
		opts = append(opts, Name(q.Names()...), ID(q.IDs()...))
		versionConstraints = append(versionConstraints, q.Versions())
	}
	return New(append(opts, Version(version.DryMerge(versionConstraints...)...))...)
}

// Name adds provided names to the aggregate names that a Query targets. It
// ensures that each name is unique within the Query. If a name already exists
// in the Query, it will not be added again. The function returns an Option to
// be used with New or other functions that accept Options.
func Name(names ...string) Option {
	return func(q *Query) {
	L:
		for _, name := range names {
			for _, name2 := range q.Q.Names {
				if name == name2 {
					continue L
				}
			}
			q.Q.Names = append(q.Q.Names, name)
		}
	}
}

// ID is an option for a Query that specifies a slice of UUIDs to filter
// Aggregates by. The provided UUIDs are added to the Query's existing list of
// IDs, with any duplicates being ignored. Only Aggregates with an ID present in
// this list will be considered when processing the Query.
func ID(ids ...uuid.UUID) Option {
	return func(q *Query) {
	L:
		for _, id := range ids {
			for _, id2 := range q.Q.IDs {
				if id == id2 {
					continue L
				}
			}
			q.Q.IDs = append(q.Q.IDs, id)
		}
	}
}

// Version appends the provided version constraints to the version constraints
// of a Query. The constraints are used to filter aggregates based on their
// versions when processing the Query. The function accepts an arbitrary number
// of version.Option as its parameters. These options define the exact, minimum,
// maximum, or range of versions that an aggregate must have in order to match
// the Query.
func Version(constraints ...version.Option) Option {
	return func(q *Query) {
		q.versionConstraints = append(q.versionConstraints, constraints...)
	}
}

// SortBy sets the sorting options for a Query. It determines how the Aggregates
// that match the Query will be sorted. SortBy takes a sort parameter of type
// [aggregate.Sorting] to specify the field to sort by, and a direction
// parameter of type [aggregate.SortDirection] to specify the direction of
// sorting. It returns an [Option] that can be used to build or modify a Query.
func SortBy(sort aggregate.Sorting, dir aggregate.SortDirection) Option {
	return func(q *Query) {
		q.Q.Sortings = []aggregate.SortOptions{{Sort: sort, Dir: dir}}
	}
}

// SortByMulti appends multiple sort options to a Query. It allows the sorting
// of aggregates in a Query based on multiple criteria. This function is an
// Option type, meaning it modifies the state of a Query when passed into the
// New function. The sort options are specified by providing one or more
// instances of aggregate.SortOptions.
func SortByMulti(sorts ...aggregate.SortOptions) Option {
	return func(q *Query) {
		q.Q.Sortings = append(q.Q.Sortings, sorts...)
	}
}

// Tagger is an interface that provides a method for determining if a specific
// tag is associated with an object. It's primarily used in the context of
// aggregate queries, where it can be implemented to filter out aggregates based
// on their tagging.
type Tagger interface {
	// HasTag checks if a given tag is associated with the [Tagger] interface. It
	// returns true if the tag exists, and false otherwise.
	HasTag(string) bool
}

type queryWithTags interface {
	aggregate.Query

	// Tags returns a slice of tag strings associated with the query. These tags can
	// be used to further refine or categorize the results returned by the query.
	Tags() []string
}

// Test filters an aggregate based on the criteria specified in the provided
// aggregate query. It checks if the name, ID, and version of the aggregate
// match the names, IDs, and versions specified in the query. If the aggregate
// also implements the [Tagger] interface and the query includes tags, Test
// checks if any of those tags are associated with the aggregate. Test returns
// true if all checks pass, otherwise it returns false.
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

// EventQueryOpts converts an [aggregate.Query] into a slice of [query.Option].
// The returned options can be used to filter events based on the names, IDs,
// and version constraints of the original aggregate query.

// EventQueryOpts is typically used to convert the aggregate query into a query
// that can be passed to an event store.
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
	return q.Q.Names
}

// IDs returns a slice of UUIDs that the Query filters by.
func (q Query) IDs() []uuid.UUID {
	return q.Q.IDs
}

// Versions returns the version constraints of the Query, which are used to
// filter aggregates based on their versions.
func (q Query) Versions() version.Constraints {
	return q.Q.Versions
}

// Sortings returns the sorting options of the Query. The returned sort options
// determine the order in which Aggregates should be sorted when processing the
// Query.
func (q Query) Sortings() []aggregate.SortOptions {
	return q.Q.Sortings
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
