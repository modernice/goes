package aggregate_test

import (
	"fmt"
	"reflect"
	"sort"
	"testing"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/internal"
	"github.com/modernice/goes/internal/xaggregate"
)

func TestSort_name(t *testing.T) {
	aggregates := []aggregate.Aggregate{
		aggregate.New("b", internal.NewUUID()),
		aggregate.New("3", internal.NewUUID()),
		aggregate.New("a", internal.NewUUID()),
		aggregate.New("d", internal.NewUUID()),
		aggregate.New("2", internal.NewUUID()),
		aggregate.New("c", internal.NewUUID()),
		aggregate.New("f", internal.NewUUID()),
		aggregate.New("1", internal.NewUUID()),
		aggregate.New("e", internal.NewUUID()),
	}

	tests := map[aggregate.SortDirection][]aggregate.Aggregate{
		aggregate.SortAsc: {
			aggregates[7],
			aggregates[4],
			aggregates[1],
			aggregates[2],
			aggregates[0],
			aggregates[5],
			aggregates[3],
			aggregates[8],
			aggregates[6],
		},
		aggregate.SortDesc: {
			aggregates[6],
			aggregates[8],
			aggregates[3],
			aggregates[5],
			aggregates[0],
			aggregates[2],
			aggregates[1],
			aggregates[4],
			aggregates[7],
		},
	}

	for dir, want := range tests {
		t.Run(fmt.Sprint(dir), func(t *testing.T) {
			got := aggregate.Sort(aggregates, aggregate.SortName, dir)
			if !reflect.DeepEqual(want, got) {
				t.Fatalf("aggregates sorted wrongly\n\nwant: %#v\n\ngot: %#v\n\n", want, got)
			}
		})
	}
}

func TestSort_id(t *testing.T) {
	aggregates := []aggregate.Aggregate{
		aggregate.New("foo", internal.NewUUID()),
		aggregate.New("foo", internal.NewUUID()),
		aggregate.New("foo", internal.NewUUID()),
		aggregate.New("foo", internal.NewUUID()),
		aggregate.New("foo", internal.NewUUID()),
		aggregate.New("foo", internal.NewUUID()),
		aggregate.New("foo", internal.NewUUID()),
		aggregate.New("foo", internal.NewUUID()),
		aggregate.New("foo", internal.NewUUID()),
	}

	ascAggregates := make([]aggregate.Aggregate, len(aggregates))
	descAggregates := make([]aggregate.Aggregate, len(aggregates))
	copy(ascAggregates, aggregates)
	copy(descAggregates, aggregates)

	sort.Slice(ascAggregates, func(i, j int) bool {
		iid, _, _ := ascAggregates[i].Aggregate()
		jid, _, _ := ascAggregates[j].Aggregate()
		return iid.String() <= jid.String()
	})

	sort.Slice(descAggregates, func(i, j int) bool {
		iid, _, _ := descAggregates[i].Aggregate()
		jid, _, _ := descAggregates[j].Aggregate()
		return iid.String() > jid.String()
	})

	tests := map[aggregate.SortDirection][]aggregate.Aggregate{
		aggregate.SortAsc:  ascAggregates,
		aggregate.SortDesc: descAggregates,
	}

	for dir, want := range tests {
		t.Run(fmt.Sprint(dir), func(t *testing.T) {
			got := aggregate.Sort(aggregates, aggregate.SortID, dir)
			if !reflect.DeepEqual(want, got) {
				t.Fatalf("aggregates sorted wrongly\n\nwant: %#v\n\ngot: %#v\n\n", want, got)
			}
		})
	}
}
func TestSort_version(t *testing.T) {
	aggregates := []aggregate.Aggregate{
		aggregate.New("foo", internal.NewUUID(), aggregate.Version(0)),
		aggregate.New("foo", internal.NewUUID(), aggregate.Version(5)),
		aggregate.New("foo", internal.NewUUID(), aggregate.Version(3)),
		aggregate.New("foo", internal.NewUUID(), aggregate.Version(7)),
		aggregate.New("foo", internal.NewUUID(), aggregate.Version(2)),
		aggregate.New("foo", internal.NewUUID(), aggregate.Version(8)),
		aggregate.New("foo", internal.NewUUID(), aggregate.Version(1)),
		aggregate.New("foo", internal.NewUUID(), aggregate.Version(6)),
		aggregate.New("foo", internal.NewUUID(), aggregate.Version(4)),
	}

	tests := map[aggregate.SortDirection][]aggregate.Aggregate{
		aggregate.SortAsc: {
			aggregates[0],
			aggregates[6],
			aggregates[4],
			aggregates[2],
			aggregates[8],
			aggregates[1],
			aggregates[7],
			aggregates[3],
			aggregates[5],
		},
		aggregate.SortDesc: {
			aggregates[5],
			aggregates[3],
			aggregates[7],
			aggregates[1],
			aggregates[8],
			aggregates[2],
			aggregates[4],
			aggregates[6],
			aggregates[0],
		},
	}

	for dir, want := range tests {
		t.Run(fmt.Sprint(dir), func(t *testing.T) {
			got := aggregate.Sort(aggregates, aggregate.SortVersion, dir)
			if !reflect.DeepEqual(want, got) {
				t.Fatalf("aggregates sorted wrongly\n\nwant: %#v\n\ngot: %#v\n\n", want, got)
			}
		})
	}
}

func TestSortMulti(t *testing.T) {
	as := []aggregate.Aggregate{
		aggregate.New("foo1", uuid.MustParse("A0000000-0000-0000-0000-000000000000"), aggregate.Version(1)),
		aggregate.New("foo1", uuid.MustParse("B0000000-0000-0000-0000-000000000000"), aggregate.Version(2)),
		aggregate.New("foo1", uuid.MustParse("C0000000-0000-0000-0000-000000000000"), aggregate.Version(3)),
		aggregate.New("foo2", uuid.MustParse("A0000000-0000-0000-0000-000000000000"), aggregate.Version(1)),
		aggregate.New("foo2", uuid.MustParse("B0000000-0000-0000-0000-000000000000"), aggregate.Version(2)),
		aggregate.New("foo2", uuid.MustParse("C0000000-0000-0000-0000-000000000000"), aggregate.Version(3)),
		aggregate.New("foo3", uuid.MustParse("A0000000-0000-0000-0000-000000000000"), aggregate.Version(1)),
		aggregate.New("foo3", uuid.MustParse("B0000000-0000-0000-0000-000000000000"), aggregate.Version(2)),
		aggregate.New("foo3", uuid.MustParse("C0000000-0000-0000-0000-000000000000"), aggregate.Version(3)),
	}

	shuffled := xaggregate.Shuffle(as)

	sorted := aggregate.SortMulti(
		shuffled,
		aggregate.SortOptions{Sort: aggregate.SortName, Dir: aggregate.SortDesc},
		aggregate.SortOptions{Sort: aggregate.SortVersion, Dir: aggregate.SortAsc},
	)

	want := []aggregate.Aggregate{
		as[6], as[7], as[8],
		as[3], as[4], as[5],
		as[0], as[1], as[2],
	}
	if !reflect.DeepEqual(want, sorted) {
		t.Errorf("aggregates got sorted incorrectly.\n\nwant: %#v\n\ngot: %#v\n\n", want, sorted)
	}

	sorted = aggregate.SortMulti(
		shuffled,
		aggregate.SortOptions{Sort: aggregate.SortName, Dir: aggregate.SortAsc},
		aggregate.SortOptions{Sort: aggregate.SortVersion, Dir: aggregate.SortDesc},
	)

	want = []aggregate.Aggregate{
		as[2], as[1], as[0],
		as[5], as[4], as[3],
		as[8], as[7], as[6],
	}
	if !reflect.DeepEqual(want, sorted) {
		t.Errorf("aggregates got sorted incorrectly.\n\nwant: %#v\n\ngot: %#v\n\n", want, sorted)
	}

	sorted = aggregate.SortMulti(
		shuffled,
		aggregate.SortOptions{Sort: aggregate.SortVersion, Dir: aggregate.SortDesc},
		aggregate.SortOptions{Sort: aggregate.SortName, Dir: aggregate.SortDesc},
	)

	want = []aggregate.Aggregate{
		as[8], as[5], as[2],
		as[7], as[4], as[1],
		as[6], as[3], as[0],
	}
	if !reflect.DeepEqual(want, sorted) {
		t.Errorf("aggregates got sorted incorrectly.\n\nwant: %#v\n\ngot: %#v\n\n", want, sorted)
	}

	sorted = aggregate.SortMulti(
		shuffled,
		aggregate.SortOptions{Sort: aggregate.SortName, Dir: aggregate.SortAsc},
		aggregate.SortOptions{Sort: aggregate.SortID, Dir: aggregate.SortDesc},
	)

	want = []aggregate.Aggregate{
		as[2], as[1], as[0],
		as[5], as[4], as[3],
		as[8], as[7], as[6],
	}
	if !reflect.DeepEqual(want, sorted) {
		t.Errorf("aggregates got sorted incorrectly.\n\nwant: %#v\n\ngot: %#v\n\n", want, sorted)
	}
}
