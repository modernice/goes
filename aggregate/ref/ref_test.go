package ref_test

import (
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/ref"
	"github.com/modernice/goes/internal"
)

func TestNames(t *testing.T) {
	tests := []struct {
		refs []aggregate.Ref
		want []string
	}{
		{refs: []aggregate.Ref{}},
		{refs: []aggregate.Ref{{}}},
		{
			refs: []aggregate.Ref{{Name: "foo"}},
			want: []string{"foo"},
		},
		{
			refs: []aggregate.Ref{{Name: "foo"}, {Name: "bar"}},
			want: []string{"foo", "bar"},
		},
		{
			refs: []aggregate.Ref{{Name: "foo"}, {Name: "bar"}, {Name: "bar"}},
			want: []string{"foo", "bar"},
		},
	}

	for _, tt := range tests {
		names := ref.Names(tt.refs...)
		if !reflect.DeepEqual(names, tt.want) {
			t.Fatalf("Names(%v) should return %v; got %v", tt.refs, tt.want, names)
		}
	}
}

func TestIDs(t *testing.T) {
	ids := []uuid.UUID{internal.NewUUID(), internal.NewUUID(), internal.NewUUID()}
	tests := []struct {
		refs []aggregate.Ref
		want []uuid.UUID
	}{
		{refs: []aggregate.Ref{}},
		{refs: []aggregate.Ref{{}}},
		{
			refs: []aggregate.Ref{{Name: "foo", ID: ids[0]}},
			want: []uuid.UUID{ids[0]},
		},
		{
			refs: []aggregate.Ref{{Name: "foo", ID: ids[0]}, {Name: "bar", ID: ids[1]}},
			want: []uuid.UUID{ids[0], ids[1]},
		},
		{
			refs: []aggregate.Ref{{Name: "foo", ID: ids[0]}, {Name: "bar", ID: ids[1]}, {Name: "bar", ID: ids[2]}},
			want: []uuid.UUID{ids[0], ids[1], ids[2]},
		},
		{
			refs: []aggregate.Ref{{Name: "foo", ID: ids[0]}, {Name: "bar", ID: ids[1]}, {Name: "baz", ID: ids[1]}},
			want: []uuid.UUID{ids[0], ids[1]},
		},
	}

	for _, tt := range tests {
		ids := ref.IDs(tt.refs...)
		if !reflect.DeepEqual(ids, tt.want) {
			t.Fatalf("IDs(%v) should return %v; got %v", tt.refs, tt.want, ids)
		}
	}
}

func TestAggregates(t *testing.T) {
	ids := []uuid.UUID{internal.NewUUID(), internal.NewUUID(), internal.NewUUID()}
	tests := []struct {
		refs []aggregate.Ref
		name string
		want []uuid.UUID
	}{
		{},
		{refs: []aggregate.Ref{{}}},
		{refs: []aggregate.Ref{{Name: "foo"}}},
		{refs: []aggregate.Ref{{Name: "foo", ID: ids[0]}}},
		{
			refs: []aggregate.Ref{{Name: "foo", ID: ids[0]}},
			name: "bar",
		},
		{
			refs: []aggregate.Ref{{Name: "foo", ID: ids[0]}},
			name: "foo",
			want: ids[:1],
		},
		{
			refs: []aggregate.Ref{
				{Name: "foo", ID: ids[0]},
				{Name: "foo", ID: ids[1]},
			},
			name: "foo",
			want: ids[:2],
		},
		{
			refs: []aggregate.Ref{
				{Name: "foo", ID: ids[0]},
				{Name: "bar", ID: ids[1]},
			},
			name: "foo",
			want: ids[:1],
		},
		{
			refs: []aggregate.Ref{
				{Name: "foo", ID: ids[0]},
				{Name: "foo", ID: ids[0]},
			},
			name: "foo",
			want: ids[:1],
		},
		{
			refs: []aggregate.Ref{
				{Name: "foo", ID: ids[0]},
				{Name: "bar", ID: ids[1]},
				{Name: "foo", ID: ids[2]},
			},
			name: "foo",
			want: []uuid.UUID{ids[0], ids[2]},
		},
	}

	for _, tt := range tests {
		ids := ref.Aggregates(tt.name, tt.refs...)
		if !reflect.DeepEqual(ids, tt.want) {
			t.Fatalf("Aggregates(%v, %q) should return %v; got %v", tt.refs, tt.name, tt.want, ids)
		}
	}
}
