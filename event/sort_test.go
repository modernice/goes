package event_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/test"
	"github.com/modernice/goes/internal/xevent"
)

func TestSort(t *testing.T) {
	now := time.Now()
	aggregateIDs := []uuid.UUID{
		uuid.MustParse("A0000000-0000-0000-0000-000000000000"),
		uuid.MustParse("B0000000-0000-0000-0000-000000000000"),
		uuid.MustParse("C0000000-0000-0000-0000-000000000000"),
		uuid.MustParse("D0000000-0000-0000-0000-000000000000"),
		uuid.MustParse("E0000000-0000-0000-0000-000000000000"),
	}
	events := []event.Event{
		event.New("foo3", test.FooEventData{}, event.Time(now), event.Aggregate("foo3", aggregateIDs[0], 3)),
		event.New("foo1", test.FooEventData{}, event.Time(now.Add(24*time.Hour)), event.Aggregate("foo1", aggregateIDs[1], 1)),
		event.New("foo5", test.FooEventData{}, event.Time(now.Add(12*time.Hour)), event.Aggregate("foo5", aggregateIDs[2], 2)),
		event.New("foo4", test.FooEventData{}, event.Time(now.Add(time.Hour)), event.Aggregate("foo4", aggregateIDs[3], 5)),
		event.New("foo2", test.FooEventData{}, event.Time(now.Add(48*time.Hour)), event.Aggregate("foo2", aggregateIDs[4], 4)),
	}

	tests := []struct {
		name string
		sort event.Sorting
		dir  event.SortDirection
		want []event.Event
	}{
		{
			name: "SortTime(asc)",
			sort: event.SortTime,
			dir:  event.SortAsc,
			want: []event.Event{events[0], events[3], events[2], events[1], events[4]},
		},
		{
			name: "SortTime(desc)",
			sort: event.SortTime,
			dir:  event.SortDesc,
			want: []event.Event{events[4], events[1], events[2], events[3], events[0]},
		},
		{
			name: "SortAggregateName(asc)",
			sort: event.SortAggregateName,
			dir:  event.SortAsc,
			want: []event.Event{events[1], events[4], events[0], events[3], events[2]},
		},
		{
			name: "SortAggregateName(desc)",
			sort: event.SortAggregateName,
			dir:  event.SortDesc,
			want: []event.Event{events[2], events[3], events[0], events[4], events[1]},
		},
		{
			name: "SortAggregateID(asc)",
			sort: event.SortAggregateID,
			dir:  event.SortAsc,
			want: []event.Event{events[0], events[1], events[2], events[3], events[4]},
		},
		{
			name: "SortAggregateID(desc)",
			sort: event.SortAggregateID,
			dir:  event.SortDesc,
			want: []event.Event{events[4], events[3], events[2], events[1], events[0]},
		},
		{
			name: "SortAggregateVersion(asc)",
			sort: event.SortAggregateVersion,
			dir:  event.SortAsc,
			want: []event.Event{events[1], events[2], events[0], events[4], events[3]},
		},
		{
			name: "SortAggregateVersion(desc)",
			sort: event.SortAggregateVersion,
			dir:  event.SortDesc,
			want: []event.Event{events[3], events[4], events[0], events[2], events[1]},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := event.Sort(events, tt.sort, tt.dir)
			test.AssertEqualEvents(t, tt.want, got)
		})
	}
}

func TestSortMulti(t *testing.T) {
	now := time.Now()
	aggregateID := uuid.New()
	events := []event.Event{
		event.New("foo", test.FooEventData{}, event.Time(now), event.Aggregate("foo", aggregateID, 1)),
		event.New("foo", test.FooEventData{}, event.Time(now), event.Aggregate("foo", aggregateID, 2)),
		event.New("foo", test.FooEventData{}, event.Time(now), event.Aggregate("foo", aggregateID, 3)),
		event.New("foo", test.FooEventData{}, event.Time(now.Add(12*time.Hour)), event.Aggregate("bar", aggregateID, 1)),
		event.New("foo", test.FooEventData{}, event.Time(now.Add(12*time.Hour)), event.Aggregate("bar", aggregateID, 2)),
		event.New("foo", test.FooEventData{}, event.Time(now.Add(12*time.Hour)), event.Aggregate("bar", aggregateID, 3)),
	}
	shuffled := xevent.Shuffle(events)

	sorted := event.SortMulti(
		shuffled,
		event.SortOptions{Sort: event.SortAggregateName, Dir: event.SortAsc},
		event.SortOptions{Sort: event.SortAggregateVersion, Dir: event.SortAsc},
	)

	test.AssertEqualEvents(t, []event.Event{
		events[3], events[4], events[5],
		events[0], events[1], events[2],
	}, sorted)

	sorted = event.SortMulti(
		shuffled,
		event.SortOptions{Sort: event.SortAggregateName, Dir: event.SortAsc},
		event.SortOptions{Sort: event.SortAggregateVersion, Dir: event.SortDesc},
	)

	test.AssertEqualEvents(t, []event.Event{
		events[5], events[4], events[3],
		events[2], events[1], events[0],
	}, sorted)

	sorted = event.SortMulti(
		shuffled,
		event.SortOptions{Sort: event.SortAggregateName, Dir: event.SortDesc},
		event.SortOptions{Sort: event.SortAggregateVersion, Dir: event.SortAsc},
	)

	test.AssertEqualEvents(t, []event.Event{
		events[0], events[1], events[2],
		events[3], events[4], events[5],
	}, sorted)
}
