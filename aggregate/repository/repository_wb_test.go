package repository

import (
	"context"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate/query"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventstore/memstore"
	equery "github.com/modernice/goes/event/query"
	"github.com/modernice/goes/event/query/version"
)

func TestRepository_makeQueryOptions(t *testing.T) {
	id := uuid.New()
	q := query.New(query.Name("foo"), query.ID(id), query.Version(version.Exact(3)))
	r := newRepository(memstore.New())
	opts, _, err := r.makeQueryOptions(context.Background(), q)
	if err != nil {
		t.Fatalf("makeQueryOptions failed with %q", err)
	}

	eq := equery.New(opts...)
	want := equery.New(
		equery.AggregateName("foo"),
		equery.AggregateID(id),
		equery.AggregateVersion(version.Max(3)),
		equery.SortByMulti(
			event.SortOptions{Sort: event.SortAggregateName, Dir: event.SortAsc},
			event.SortOptions{Sort: event.SortAggregateID, Dir: event.SortAsc},
			event.SortOptions{Sort: event.SortAggregateVersion, Dir: event.SortAsc},
		),
	)

	if !reflect.DeepEqual(eq, want) {
		t.Errorf("makeQueryOptions returned the wrong Query\n\nwant: %#v\n\ngot: %#v\n\n", want, eq)
	}
}
