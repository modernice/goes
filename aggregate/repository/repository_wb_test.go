package repository

import (
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate/query"
	"github.com/modernice/goes/event"
	equery "github.com/modernice/goes/event/query"
	"github.com/modernice/goes/event/query/version"
)

func TestMakeQueryOptions(t *testing.T) {
	id := uuid.New()
	q := query.New(query.Name("foo"), query.ID(id), query.Version(version.Exact(3)))
	opts := makeQueryOptions(q)

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
