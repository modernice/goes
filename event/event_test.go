package event_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/test"
	"github.com/modernice/goes/helper/pick"
	"github.com/modernice/goes/internal"
	"github.com/modernice/goes/internal/xtime"
)

type mockData struct {
	FieldA string
	FieldB bool
}

func TestNew(t *testing.T) {
	data := newMockData()
	evt := event.New("foo", data)

	if evt.ID() == uuid.Nil {
		t.Errorf("evt.ID() shouldn't be zero-value; got %q", evt.ID())
	}

	if evt.Name() != "foo" {
		t.Errorf("evt.Name() should return %q; got %q", "foo", evt.Name())
	}

	if evt.Data() != data {
		t.Errorf("evt.Data() should return %#v; got %#v", data, evt.Data())
	}

	if d := time.Since(evt.Time()); d > 100*time.Millisecond {
		t.Errorf("evt.Time() should almost equal %s; got %s", xtime.Now(), evt.Time())
	}

	if pick.AggregateName(evt) != "" {
		t.Errorf("evt.AggregateName() should return %q; got %q", "", pick.AggregateName(evt))
	}

	if pick.AggregateID(evt) != uuid.Nil {
		t.Errorf("evt.AggregateID() should return %q; git %q", uuid.Nil, evt.ID())
	}

	if pick.AggregateVersion(evt) != 0 {
		t.Errorf("evt.AggrgateVersion() should return %v; got %v", 0, pick.AggregateVersion(evt))
	}
}

func TestNew_time(t *testing.T) {
	ts := xtime.Now().Add(time.Hour)
	evt := event.New("foo", newMockData(), event.Time(ts))
	if !ts.Equal(evt.Time()) {
		t.Errorf("expected evt.Time() to equal %s; got %s", ts, evt.Time())
	}
}

func TestNew_aggregate(t *testing.T) {
	aname := "bar"
	aid := internal.NewUUID()
	v := 3

	evt := event.New("foo", newMockData(), event.Aggregate(aid, aname, v))
	if pick.AggregateName(evt) != "bar" {
		t.Errorf("expected pick.AggregateName(evt) to return %q; got %q", "bar", pick.AggregateName(evt))
	}

	if pick.AggregateID(evt) != aid {
		t.Errorf("expected pick.AggregateID(evt) to return %q; got %q", aid, pick.AggregateID(evt))
	}

	if pick.AggregateVersion(evt) != v {
		t.Errorf("expected pick.AggregateVersion(evt) to return %v; got %v", v, pick.AggregateVersion(evt))
	}
}

func TestNew_previous(t *testing.T) {
	aggregateID := internal.NewUUID()
	prev := event.New("foo", test.FooEventData{A: "foo"}, event.Aggregate(aggregateID, "foobar", 3))
	evt := event.New("bar", test.BarEventData{A: "bar"}, event.Previous[test.FooEventData](prev))

	if evt.Name() != "bar" {
		t.Errorf("expected evt.Name to return %q; got %q", "bar", evt.Name())
	}

	wantData := test.BarEventData{A: "bar"}
	if evt.Data() != wantData {
		t.Errorf("expected evt.Data to return %#v; got %#v", wantData, evt.Data())
	}

	if pick.AggregateName(evt) != "foobar" {
		t.Errorf("expected evt.AggregateName to return %q; got %q", "foobar", pick.AggregateName(evt))
	}

	if pick.AggregateID(evt) != aggregateID {
		t.Errorf("expected evt.AggregateID to return %q; got %q", aggregateID, pick.AggregateID(evt))
	}

	if pick.AggregateVersion(evt) != 4 {
		t.Errorf("expected evt.AggregateVersion to return %d; got %d", 4, pick.AggregateVersion(evt))
	}
}

func TestEqual(t *testing.T) {
	id := internal.NewUUID()
	now := xtime.Now()
	tests := []struct {
		a    event.Evt[mockData]
		b    event.Evt[mockData]
		want bool
	}{
		{
			a:    event.New("foo", mockData{FieldA: "foo"}, event.ID(id), event.Time(now)),
			b:    event.New("foo", mockData{FieldA: "foo"}, event.ID(id), event.Time(now)),
			want: true,
		},
		{
			a:    event.New("foo", mockData{FieldA: "foo"}, event.ID(id)),
			b:    event.New("foo", mockData{FieldA: "foo"}, event.ID(id), event.Time(now)),
			want: false,
		},
		{
			a:    event.New("foo", mockData{FieldA: "foo"}),
			b:    event.New("foo", mockData{FieldA: "foo"}, event.ID(id)),
			want: false,
		},
		{
			a:    event.New("foo", mockData{FieldA: "foo"}),
			b:    event.New("foo", mockData{FieldA: "foo"}),
			want: false,
		},
		{
			a:    event.New("foo", mockData{FieldA: "foo"}, event.ID(id), event.Time(now)),
			b:    event.New("bar", mockData{FieldA: "foo"}, event.ID(id), event.Time(now)),
			want: false,
		},
		{
			a:    event.New("foo", mockData{FieldA: "foo"}, event.ID(id), event.Time(now)),
			b:    event.New("foo", mockData{FieldA: "bar"}, event.ID(id), event.Time(now)),
			want: false,
		},
		{
			a:    event.New("foo", mockData{FieldA: "foo"}, event.ID(id), event.Time(now)),
			b:    event.New("foo", mockData{FieldA: "foo"}, event.ID(id), event.Time(now), event.ID(internal.NewUUID())),
			want: false,
		},
		{
			a:    event.New("foo", mockData{FieldA: "foo"}, event.ID(id), event.Time(now)),
			b:    event.New("foo", mockData{FieldA: "foo"}, event.ID(id), event.Time(now), event.Aggregate(internal.NewUUID(), "foobar", 2)),
			want: false,
		},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			if event.Equal(tt.a.Any(), tt.b.Any()) != tt.want {
				if tt.want {
					t.Errorf("expected events to be equal but they aren't\nevent a: %#v\n\nevent b: %#v", tt.a, tt.b)
					return
				}
				t.Errorf("expected events not to be equal but they are\nevent a: %#v\n\nevent b: %#v", tt.a, tt.b)
			}
		})
	}
}

func TestEqual_variadic(t *testing.T) {
	id := internal.NewUUID()
	now := xtime.Now()
	events := []event.Event{
		event.New("foo", newMockData(), event.ID(id), event.Time(now)).Any(),
		event.New("foo", newMockData(), event.ID(id), event.Time(now)).Any(),
		event.New("foo", newMockData(), event.ID(id), event.Time(now)).Any(),
	}

	if !event.Equal(events...) {
		t.Error(fmt.Errorf("expected events to be equal but they aren't\n%#v", events))
	}

	events = append(events, event.New("bar", newMockData(), event.ID(id), event.Time(now)).Any())

	if event.Equal(events...) {
		t.Error(fmt.Errorf("expected events not to be equal but they are\n%#v", events))
	}
}

func newMockData() mockData {
	return mockData{FieldA: "foo", FieldB: true}
}
