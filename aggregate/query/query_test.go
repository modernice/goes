package query

import (
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/event"
	equery "github.com/modernice/goes/event/query"
	"github.com/modernice/goes/event/query/version"
	"github.com/modernice/goes/internal"
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
				Q: Q{
					Names:    []string{"foo", "bar", "baz", "foobar"},
					Versions: version.Filter(),
				},
			},
		},
		{
			name: "ID",
			opts: []Option{
				ID(ids[:2]...),
				ID(ids[2:4]...),
			},
			want: Query{
				Q: Q{
					IDs:      ids,
					Versions: version.Filter(),
				},
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
				Q: Q{
					Versions: version.Filter(
						version.Exact(1, 2, 3),
						version.InRange(version.Range{0, 100}),
						version.Min(4),
						version.Max(20),
					),
				},
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

func TestMerge(t *testing.T) {
	ids := []uuid.UUID{
		internal.NewUUID(),
		internal.NewUUID(),
		internal.NewUUID(),
		internal.NewUUID(),
	}

	queries := []aggregate.Query{
		New(
			Name("foo", "bar"),
			ID(ids[:2]...),
			Version(version.Exact(1, 2), version.Min(4)),
		),
		New(
			Name("foobar", "barbaz"),
			ID(ids[1:3]...),
			Version(version.Exact(3, 4), version.Max(9)),
		),
		New(
			Name("foobar", "barbaz"),
			ID(ids[1:3]...),
			Version(version.Exact(3, 4), version.Max(9)),
		),
	}

	q := Merge(queries...)
	want := Query{
		Q: Q{
			IDs:      ids[:3],
			Names:    []string{"foo", "bar", "foobar", "barbaz"},
			Versions: version.Filter(version.Exact(1, 2, 3, 4), version.Min(4), version.Max(9)),
		},
	}

	if !reflect.DeepEqual(q, want) {
		t.Fatalf("Merge should return\n\n%#v\n\ngot\n\n%#v", want, q)
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
			want: equery.New(),
		},
		{
			name: "Name",
			give: New(Name("foo", "bar", "baz")),
			want: equery.New(equery.AggregateName("foo", "bar", "baz")),
		},
		{
			name: "ID",
			give: New(ID(ids...)),
			want: equery.New(equery.AggregateID(ids...)),
		},
		{
			name: "Version(exact)",
			give: New(Version(version.Exact(1, 2, 3))),
			want: equery.New(equery.AggregateVersion(version.Max(1, 2, 3))),
		},
		{
			name: "Version(range)",
			give: New(Version(version.InRange(version.Range{10, 70}, version.Range{30, 50}))),
			want: equery.New(equery.AggregateVersion(version.Max(70, 50))),
		},
		{
			name: "Version(min)",
			give: New(Version(version.Min(2, 4, 6))),
			want: equery.New(),
		},
		{
			name: "Version(max)",
			give: New(Version(version.Max(20, 40, 60))),
			want: equery.New(equery.AggregateVersion(version.Max(20, 40, 60))),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := EventQueryOpts(tt.give)
			got := equery.New(opts...)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ToEventQuery returned a wrong Query:\n\nwant: %#v\n\ngot: %#v", tt.want, got)
			}
		})
	}
}

func makeUUIDs(n int) []uuid.UUID {
	ids := make([]uuid.UUID, n)
	for i := range ids {
		ids[i] = internal.NewUUID()
	}
	return ids
}
