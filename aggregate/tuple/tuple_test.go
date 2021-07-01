package tuple_test

import (
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/tuple"
)

func TestNames(t *testing.T) {
	tests := []struct {
		tuples []aggregate.Tuple
		want   []string
	}{
		{tuples: []aggregate.Tuple{}},
		{tuples: []aggregate.Tuple{{}}},
		{
			tuples: []aggregate.Tuple{{Name: "foo"}},
			want:   []string{"foo"},
		},
		{
			tuples: []aggregate.Tuple{{Name: "foo"}, {Name: "bar"}},
			want:   []string{"foo", "bar"},
		},
		{
			tuples: []aggregate.Tuple{{Name: "foo"}, {Name: "bar"}, {Name: "bar"}},
			want:   []string{"foo", "bar"},
		},
	}

	for _, tt := range tests {
		names := tuple.Names(tt.tuples...)
		if !reflect.DeepEqual(names, tt.want) {
			t.Fatalf("Names(%v) should return %v; got %v", tt.tuples, tt.want, names)
		}
	}
}

func TestIDs(t *testing.T) {
	ids := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	tests := []struct {
		tuples []aggregate.Tuple
		want   []uuid.UUID
	}{
		{tuples: []aggregate.Tuple{}},
		{tuples: []aggregate.Tuple{{}}},
		{
			tuples: []aggregate.Tuple{{Name: "foo", ID: ids[0]}},
			want:   []uuid.UUID{ids[0]},
		},
		{
			tuples: []aggregate.Tuple{{Name: "foo", ID: ids[0]}, {Name: "bar", ID: ids[1]}},
			want:   []uuid.UUID{ids[0], ids[1]},
		},
		{
			tuples: []aggregate.Tuple{{Name: "foo", ID: ids[0]}, {Name: "bar", ID: ids[1]}, {Name: "bar", ID: ids[2]}},
			want:   []uuid.UUID{ids[0], ids[1], ids[2]},
		},
		{
			tuples: []aggregate.Tuple{{Name: "foo", ID: ids[0]}, {Name: "bar", ID: ids[1]}, {Name: "baz", ID: ids[1]}},
			want:   []uuid.UUID{ids[0], ids[1]},
		},
	}

	for _, tt := range tests {
		ids := tuple.IDs(tt.tuples...)
		if !reflect.DeepEqual(ids, tt.want) {
			t.Fatalf("IDs(%v) should return %v; got %v", tt.tuples, tt.want, ids)
		}
	}
}

func TestAggregates(t *testing.T) {
	ids := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	tests := []struct {
		tuples []aggregate.Tuple
		name   string
		want   []uuid.UUID
	}{
		{},
		{tuples: []aggregate.Tuple{{}}},
		{tuples: []aggregate.Tuple{{Name: "foo"}}},
		{tuples: []aggregate.Tuple{{Name: "foo", ID: ids[0]}}},
		{
			tuples: []aggregate.Tuple{{Name: "foo", ID: ids[0]}},
			name:   "bar",
		},
		{
			tuples: []aggregate.Tuple{{Name: "foo", ID: ids[0]}},
			name:   "foo",
			want:   ids[:1],
		},
		{
			tuples: []aggregate.Tuple{
				{Name: "foo", ID: ids[0]},
				{Name: "foo", ID: ids[1]},
			},
			name: "foo",
			want: ids[:2],
		},
		{
			tuples: []aggregate.Tuple{
				{Name: "foo", ID: ids[0]},
				{Name: "bar", ID: ids[1]},
			},
			name: "foo",
			want: ids[:1],
		},
		{
			tuples: []aggregate.Tuple{
				{Name: "foo", ID: ids[0]},
				{Name: "foo", ID: ids[0]},
			},
			name: "foo",
			want: ids[:1],
		},
		{
			tuples: []aggregate.Tuple{
				{Name: "foo", ID: ids[0]},
				{Name: "bar", ID: ids[1]},
				{Name: "foo", ID: ids[2]},
			},
			name: "foo",
			want: []uuid.UUID{ids[0], ids[2]},
		},
	}

	for _, tt := range tests {
		ids := tuple.Aggregates(tt.name, tt.tuples...)
		if !reflect.DeepEqual(ids, tt.want) {
			t.Fatalf("Aggregates(%v, %q) should return %v; got %v", tt.tuples, tt.name, tt.want, ids)
		}
	}
}
