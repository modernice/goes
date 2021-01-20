package event_test

import (
	"testing"
	"time"

	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/test"
)

func TestSort(t *testing.T) {
	now := time.Now()
	events := []event.Event{
		event.New("foo", test.FooEventData{}, event.Time(now)),
		event.New("foo", test.FooEventData{}, event.Time(now.Add(24*time.Hour))),
		event.New("foo", test.FooEventData{}, event.Time(now.Add(12*time.Hour))),
		event.New("foo", test.FooEventData{}, event.Time(now.Add(time.Hour))),
		event.New("foo", test.FooEventData{}, event.Time(now.Add(48*time.Hour))),
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := event.Sort(events, tt.sort, tt.dir)
			test.AssertEqualEvents(t, tt.want, got)
		})
	}
}
