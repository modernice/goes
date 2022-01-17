package query_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/query"
	"github.com/modernice/goes/event/query/version"
)

func TestTest(t *testing.T) {
	ids := makeUUIDs(4)

	tests := []struct {
		name  string
		query aggregate.Query
		tests map[aggregate.Aggregate[any]]bool
	}{
		{
			name:  "Name",
			query: query.New(query.Name("foo", "bar")),
			tests: map[aggregate.Aggregate[any]]bool{
				aggregate.New[any]("foo", uuid.New()): true,
				aggregate.New[any]("bar", uuid.New()): true,
				aggregate.New[any]("baz", uuid.New()): false,
			},
		},
		{
			name:  "ID",
			query: query.New(query.ID(ids[0], ids[2])),
			tests: map[aggregate.Aggregate[any]]bool{
				aggregate.New[any]("foo", ids[0]): true,
				aggregate.New[any]("bar", ids[1]): false,
				aggregate.New[any]("baz", ids[2]): true,
				aggregate.New[any]("foo", ids[3]): false,
			},
		},
		{
			name:  "Version (exact)",
			query: query.New(query.Version(version.Exact(2, 3))),
			tests: map[aggregate.Aggregate[any]]bool{
				aggregate.New[any]("foo", uuid.New(), aggregate.Version[any](1)): false,
				aggregate.New[any]("bar", uuid.New(), aggregate.Version[any](2)): true,
				aggregate.New[any]("baz", uuid.New(), aggregate.Version[any](3)): true,
			},
		},
		{
			name:  "Version (range)",
			query: query.New(query.Version(version.InRange(version.Range{1, 2}))),
			tests: map[aggregate.Aggregate[any]]bool{
				aggregate.New[any]("foo", uuid.New(), aggregate.Version[any](1)): true,
				aggregate.New[any]("bar", uuid.New(), aggregate.Version[any](2)): true,
				aggregate.New[any]("baz", uuid.New(), aggregate.Version[any](3)): false,
			},
		},
		{
			name:  "Version (min/max)",
			query: query.New(query.Version(version.Min(2), version.Max(3))),
			tests: map[aggregate.Aggregate[any]]bool{
				aggregate.New[any]("foo", uuid.New(), aggregate.Version[any](1)): false,
				aggregate.New[any]("bar", uuid.New(), aggregate.Version[any](2)): true,
				aggregate.New[any]("baz", uuid.New(), aggregate.Version[any](3)): true,
			},
		},
		{
			name: "Version (mixed)",
			query: query.New(query.Version(
				version.Min(2),
				version.Max(5),
				version.InRange(version.Range{3, 5}),
				version.Exact(1, 2, 3, 4),
			)),
			tests: map[aggregate.Aggregate[any]]bool{
				aggregate.New[any]("foo", uuid.New(), aggregate.Version[any](1)): false,
				aggregate.New[any]("bar", uuid.New(), aggregate.Version[any](2)): false,
				aggregate.New[any]("baz", uuid.New(), aggregate.Version[any](3)): true,
				aggregate.New[any]("baz", uuid.New(), aggregate.Version[any](4)): true,
				aggregate.New[any]("baz", uuid.New(), aggregate.Version[any](5)): false,
				aggregate.New[any]("baz", uuid.New(), aggregate.Version[any](6)): false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for a, want := range tt.tests {
				if got := query.Test(tt.query, a); got != want {
					t.Errorf("Test should return %t; got %t", want, got)
				}
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
