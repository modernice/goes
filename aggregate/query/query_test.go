package query

import (
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/query"
	"github.com/modernice/goes/event/query/version"
)

var _ aggregate.Query = Query{}

func TestNew(t *testing.T) {
	ids := makeUUIDs(4)

	tests := []struct {
		name string
		opts []Option
		want Query
	}{
		{
			name: "Name",
			opts: []Option{
				Name("foo", "bar"),
				Name("baz", "foobar"),
			},
			want: Query{
				names:    []string{"foo", "bar", "baz", "foobar"},
				versions: version.Filter(),
			},
		},
		{
			name: "ID",
			opts: []Option{
				ID(ids[:2]...),
				ID(ids[2:4]...),
			},
			want: Query{
				ids:      ids,
				versions: version.Filter(),
			},
		},
		{
			name: "Version",
			opts: []Option{
				Version(
					version.Exact(1, 2, 3),
					version.InRange(version.Range{0, 100}),
					version.Min(4),
					version.Max(20),
				),
			},
			want: Query{
				versions: version.Filter(
					version.Exact(1, 2, 3),
					version.InRange(version.Range{0, 100}),
					version.Min(4),
					version.Max(20),
				),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := New(tt.opts...)
			if !reflect.DeepEqual(q, tt.want) {
				t.Errorf("New returned wrong query\n\nwant: %#v\n\ngot: %#v\n\n", tt.want, q)
			}
		})
	}
}

func TestTest(t *testing.T) {
	ids := makeUUIDs(4)

	tests := []struct {
		name  string
		query aggregate.Query
		tests map[aggregate.Aggregate]bool
	}{
		{
			name:  "Name",
			query: New(Name("foo", "bar")),
			tests: map[aggregate.Aggregate]bool{
				aggregate.New("foo", uuid.New()): true,
				aggregate.New("bar", uuid.New()): true,
				aggregate.New("baz", uuid.New()): false,
			},
		},
		{
			name:  "ID",
			query: New(ID(ids[0], ids[2])),
			tests: map[aggregate.Aggregate]bool{
				aggregate.New("foo", ids[0]): true,
				aggregate.New("bar", ids[1]): false,
				aggregate.New("baz", ids[2]): true,
				aggregate.New("foo", ids[3]): false,
			},
		},
		{
			name:  "Version (exact)",
			query: New(Version(version.Exact(2, 3))),
			tests: map[aggregate.Aggregate]bool{
				aggregate.New("foo", uuid.New(), aggregate.Version(1)): false,
				aggregate.New("bar", uuid.New(), aggregate.Version(2)): true,
				aggregate.New("baz", uuid.New(), aggregate.Version(3)): true,
			},
		},
		{
			name:  "Version (range)",
			query: New(Version(version.InRange(version.Range{1, 2}))),
			tests: map[aggregate.Aggregate]bool{
				aggregate.New("foo", uuid.New(), aggregate.Version(1)): true,
				aggregate.New("bar", uuid.New(), aggregate.Version(2)): true,
				aggregate.New("baz", uuid.New(), aggregate.Version(3)): false,
			},
		},
		{
			name:  "Version (min/max)",
			query: New(Version(version.Min(2), version.Max(3))),
			tests: map[aggregate.Aggregate]bool{
				aggregate.New("foo", uuid.New(), aggregate.Version(1)): false,
				aggregate.New("bar", uuid.New(), aggregate.Version(2)): true,
				aggregate.New("baz", uuid.New(), aggregate.Version(3)): true,
			},
		},
		{
			name: "Version (mixed)",
			query: New(Version(
				version.Min(2),
				version.Max(5),
				version.InRange(version.Range{3, 5}),
				version.Exact(1, 2, 3, 4),
			)),
			tests: map[aggregate.Aggregate]bool{
				aggregate.New("foo", uuid.New(), aggregate.Version(1)): false,
				aggregate.New("bar", uuid.New(), aggregate.Version(2)): false,
				aggregate.New("baz", uuid.New(), aggregate.Version(3)): true,
				aggregate.New("baz", uuid.New(), aggregate.Version(4)): true,
				aggregate.New("baz", uuid.New(), aggregate.Version(5)): false,
				aggregate.New("baz", uuid.New(), aggregate.Version(6)): false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for a, want := range tt.tests {
				if got := Test(tt.query, a); got != want {
					t.Errorf("Test should return %t; got %t", want, got)
				}
			}
		})
	}
}

func TestEventQueryOpts(t *testing.T) {
	ids := makeUUIDs(3)
	tests := []struct {
		name string
		give Query
		want event.Query
	}{
		{
			name: "empty",
			give: New(),
			want: query.New(),
		},
		{
			name: "Name",
			give: New(Name("foo", "bar", "baz")),
			want: query.New(query.AggregateName("foo", "bar", "baz")),
		},
		{
			name: "ID",
			give: New(ID(ids...)),
			want: query.New(query.AggregateID(ids...)),
		},
		{
			name: "Version(exact)",
			give: New(Version(version.Exact(1, 2, 3))),
			want: query.New(query.AggregateVersion(version.Max(1, 2, 3))),
		},
		{
			name: "Version(range)",
			give: New(Version(version.InRange(version.Range{10, 70}, version.Range{30, 50}))),
			want: query.New(query.AggregateVersion(version.Max(70, 50))),
		},
		{
			name: "Version(min)",
			give: New(Version(version.Min(2, 4, 6))),
			want: query.New(),
		},
		{
			name: "Version(max)",
			give: New(Version(version.Max(20, 40, 60))),
			want: query.New(query.AggregateVersion(version.Max(20, 40, 60))),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := EventQueryOpts(tt.give)
			got := query.New(opts...)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ToEventQuery returned a wrong Query:\n\nwant: %#v\n\ngot: %#v", tt.want, got)
			}
		})
	}
}

func makeUUIDs(n int) []uuid.UUID {
	ids := make([]uuid.UUID, n)
	for i := range ids {
		ids[i] = uuid.New()
	}
	return ids
}
