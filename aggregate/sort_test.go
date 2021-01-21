package aggregate_test

import (
	"fmt"
	"reflect"
	"sort"
	"testing"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
)

func TestSort_name(t *testing.T) {
	aggregates := []aggregate.Aggregate{
		aggregate.New("b", uuid.New()),
		aggregate.New("3", uuid.New()),
		aggregate.New("a", uuid.New()),
		aggregate.New("d", uuid.New()),
		aggregate.New("2", uuid.New()),
		aggregate.New("c", uuid.New()),
		aggregate.New("f", uuid.New()),
		aggregate.New("1", uuid.New()),
		aggregate.New("e", uuid.New()),
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
		aggregate.New("foo", uuid.New()),
		aggregate.New("foo", uuid.New()),
		aggregate.New("foo", uuid.New()),
		aggregate.New("foo", uuid.New()),
		aggregate.New("foo", uuid.New()),
		aggregate.New("foo", uuid.New()),
		aggregate.New("foo", uuid.New()),
		aggregate.New("foo", uuid.New()),
		aggregate.New("foo", uuid.New()),
	}

	ascAggregates := make([]aggregate.Aggregate, len(aggregates))
	descAggregates := make([]aggregate.Aggregate, len(aggregates))
	copy(ascAggregates, aggregates)
	copy(descAggregates, aggregates)

	sort.Slice(ascAggregates, func(i, j int) bool {
		return ascAggregates[i].AggregateID().String() <=
			ascAggregates[j].AggregateID().String()
	})

	sort.Slice(descAggregates, func(i, j int) bool {
		return descAggregates[i].AggregateID().String() >
			descAggregates[j].AggregateID().String()
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
		aggregate.New("foo", uuid.New(), aggregate.Version(0)),
		aggregate.New("foo", uuid.New(), aggregate.Version(5)),
		aggregate.New("foo", uuid.New(), aggregate.Version(3)),
		aggregate.New("foo", uuid.New(), aggregate.Version(7)),
		aggregate.New("foo", uuid.New(), aggregate.Version(2)),
		aggregate.New("foo", uuid.New(), aggregate.Version(8)),
		aggregate.New("foo", uuid.New(), aggregate.Version(1)),
		aggregate.New("foo", uuid.New(), aggregate.Version(6)),
		aggregate.New("foo", uuid.New(), aggregate.Version(4)),
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
