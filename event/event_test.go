package event_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/test"
	"github.com/modernice/goes/helper/pick"
	"github.com/modernice/goes/internal/xtime"
)

type mockData struct {
	FieldA string
	FieldB bool
}

func TestNew(t *testing.T) {
	data := newMockData()
	evt := event.New(uuid.New(), "foo", data)

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

	if pick.AggregateName[uuid.UUID](evt) != "" {
		t.Errorf("evt.AggregateName() should return %q; got %q", "", pick.AggregateName[uuid.UUID](evt))
	}

	if pick.AggregateID[uuid.UUID](evt) != uuid.Nil {
		t.Errorf("evt.AggregateID() should return %q; git %q", uuid.Nil, evt.ID())
	}

	if pick.AggregateVersion[uuid.UUID](evt) != 0 {
		t.Errorf("evt.AggrgateVersion() should return %v; got %v", 0, pick.AggregateVersion[uuid.UUID](evt))
	}
}

func TestNew_time(t *testing.T) {
	ts := xtime.Now().Add(time.Hour)
	evt := event.New(uuid.New(), "foo", newMockData(), event.Time(ts))
	if !ts.Equal(evt.Time()) {
		t.Errorf("expected evt.Time() to equal %s; got %s", ts, evt.Time())
	}
}

func TestNew_aggregate(t *testing.T) {
	aname := "bar"
	aid := uuid.New()
	v := 3

	evt := event.New(uuid.New(), "foo", newMockData(), event.Aggregate(aid, aname, v))
	if pick.AggregateName[uuid.UUID](evt) != "bar" {
		t.Errorf("expected pick.AggregateName[uuid.UUID](evt) to return %q; got %q", "bar", pick.AggregateName[uuid.UUID](evt))
	}

	if pick.AggregateID[uuid.UUID](evt) != aid {
		t.Errorf("expected pick.AggregateID[uuid.UUID](evt) to return %q; got %q", aid, pick.AggregateID[uuid.UUID](evt))
	}

	if pick.AggregateVersion[uuid.UUID](evt) != v {
		t.Errorf("expected pick.AggregateVersion[uuid.UUID](evt) to return %v; got %v", v, pick.AggregateVersion[uuid.UUID](evt))
	}
}

func TestNew_previous(t *testing.T) {
	aggregateID := uuid.New()
	prev := event.New(uuid.New(), "foo", test.FooEventData{A: "foo"}, event.Aggregate(aggregateID, "foobar", 3))
	evt := event.New(uuid.New(), "bar", test.BarEventData{A: "bar"}, event.Previous[test.FooEventData, uuid.UUID](prev))

	if evt.Name() != "bar" {
		t.Errorf("expected evt.Name to return %q; got %q", "bar", evt.Name())
	}

	wantData := test.BarEventData{A: "bar"}
	if evt.Data() != wantData {
		t.Errorf("expected evt.Data to return %#v; got %#v", wantData, evt.Data())
	}

	if pick.AggregateName[uuid.UUID](evt) != "foobar" {
		t.Errorf("expected evt.AggregateName to return %q; got %q", "foobar", pick.AggregateName[uuid.UUID](evt))
	}

	if pick.AggregateID[uuid.UUID](evt) != aggregateID {
		t.Errorf("expected evt.AggregateID to return %q; got %q", aggregateID, pick.AggregateID[uuid.UUID](evt))
	}

	if pick.AggregateVersion[uuid.UUID](evt) != 4 {
		t.Errorf("expected evt.AggregateVersion to return %d; got %d", 4, pick.AggregateVersion[uuid.UUID](evt))
	}
}

func TestEqual(t *testing.T) {
	id := uuid.New()
	now := xtime.Now()
	tests := []struct {
		a    event.Of[mockData, uuid.UUID]
		b    event.Of[mockData, uuid.UUID]
		want bool
	}{
		{
			a:    event.New(uuid.New(), "foo", mockData{FieldA: "foo"}, event.ID(id), event.Time(now)),
			b:    event.New(uuid.New(), "foo", mockData{FieldA: "foo"}, event.ID(id), event.Time(now)),
			want: true,
		},
		{
			a:    event.New(uuid.New(), "foo", mockData{FieldA: "foo"}, event.ID(id)),
			b:    event.New(uuid.New(), "foo", mockData{FieldA: "foo"}, event.ID(id), event.Time(now)),
			want: false,
		},
		{
			a:    event.New(uuid.New(), "foo", mockData{FieldA: "foo"}),
			b:    event.New(uuid.New(), "foo", mockData{FieldA: "foo"}, event.ID(id)),
			want: false,
		},
		{
			a:    event.New(uuid.New(), "foo", mockData{FieldA: "foo"}),
			b:    event.New(uuid.New(), "foo", mockData{FieldA: "foo"}),
			want: false,
		},
		{
			a:    event.New(uuid.New(), "foo", mockData{FieldA: "foo"}, event.ID(id), event.Time(now)),
			b:    event.New(uuid.New(), "bar", mockData{FieldA: "foo"}, event.ID(id), event.Time(now)),
			want: false,
		},
		{
			a:    event.New(uuid.New(), "foo", mockData{FieldA: "foo"}, event.ID(id), event.Time(now)),
			b:    event.New(uuid.New(), "foo", mockData{FieldA: "bar"}, event.ID(id), event.Time(now)),
			want: false,
		},
		{
			a:    event.New(uuid.New(), "foo", mockData{FieldA: "foo"}, event.ID(id), event.Time(now)),
			b:    event.New(uuid.New(), "foo", mockData{FieldA: "foo"}, event.ID(id), event.Time(now), event.ID(uuid.New())),
			want: false,
		},
		{
			a:    event.New(uuid.New(), "foo", mockData{FieldA: "foo"}, event.ID(id), event.Time(now)),
			b:    event.New(uuid.New(), "foo", mockData{FieldA: "foo"}, event.ID(id), event.Time(now), event.Aggregate(uuid.New(), "foobar", 2)),
			want: false,
		},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			if event.Equal(tt.a, tt.b) != tt.want {
				if tt.want {
					t.Errorf("expected events to be equal but they aren't\nevent a: %#v\n\nevent b: %#v\n\n%s", tt.a, tt.b, cmp.Diff(tt.a, tt.b))
					return
				}
				t.Errorf("expected events not to be equal but they are\nevent a: %#v\n\nevent b: %#v", tt.a, tt.b)
			}
		})
	}
}

func TestEqual_variadic(t *testing.T) {
	id := uuid.New()
	now := xtime.Now()
	events := []event.Of[mockData, uuid.UUID]{
		event.New(uuid.New(), "foo", newMockData(), event.ID(id), event.Time(now)),
		event.New(uuid.New(), "foo", newMockData(), event.ID(id), event.Time(now)),
		event.New(uuid.New(), "foo", newMockData(), event.ID(id), event.Time(now)),
	}

	if !event.Equal(events...) {
		t.Error(fmt.Errorf("expected events to be equal but they aren't\n%#v", events))
	}

	events = append(events, event.New(uuid.New(), "bar", newMockData(), event.ID(id), event.Time(now)))

	if event.Equal(events...) {
		t.Error(fmt.Errorf("expected events not to be equal but they are\n%#v", events))
	}
}

func newMockData() mockData {
	return mockData{FieldA: "foo", FieldB: true}
}
