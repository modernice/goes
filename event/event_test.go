package event_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/event"
)

type mockData struct {
	FieldA string
	FieldB bool
}

func TestNewEvent(t *testing.T) {
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

	if d := time.Now().Sub(evt.Time()); d > 100*time.Millisecond {
		t.Errorf("evt.Time() should almost equal %s; got %s", time.Now(), evt.Time())
	}

	if evt.AggregateName() != "" {
		t.Errorf("evt.AggregateName() should return %q; got %q", "", evt.AggregateName())
	}

	if evt.AggregateID() != uuid.Nil {
		t.Errorf("evt.AggregateID() should return %q; git %q", uuid.Nil, evt.ID())
	}

	if evt.AggregateVersion() != 0 {
		t.Errorf("evt.AggrgateVersion() should return %v; got %v", 0, evt.AggregateVersion())
	}
}

func TestNewEvent_time(t *testing.T) {
	ts := time.Now().Add(time.Hour)
	evt := event.New("foo", newMockData(), event.Time(ts))
	if !ts.Equal(evt.Time()) {
		t.Errorf("expected evt.Time() to equal %s; got %s", ts, evt.Time())
	}
}

func TestNewEvent_aggregate(t *testing.T) {
	aname := "bar"
	aid := uuid.New()
	v := 3

	evt := event.New("foo", newMockData(), event.Aggregate(aname, aid, v))
	if evt.AggregateName() != "bar" {
		t.Errorf("expected evt.AggregateName() to return %q; got %q", "bar", evt.AggregateName())
	}

	if evt.AggregateID() != aid {
		t.Errorf("expected evt.AggregateID() to return %q; got %q", aid, evt.AggregateID())
	}

	if evt.AggregateVersion() != v {
		t.Errorf("expected evt.AggregateVersion() to return %v; got %v", v, evt.AggregateVersion())
	}
}

func TestEqual(t *testing.T) {
	id := uuid.New()
	now := time.Now()
	tests := []struct {
		a    event.Event
		b    event.Event
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
			b:    event.New("foo", mockData{FieldA: "foo"}, event.ID(id), event.Time(now), event.ID(uuid.New())),
			want: false,
		},
		{
			a:    event.New("foo", mockData{FieldA: "foo"}, event.ID(id), event.Time(now)),
			b:    event.New("foo", mockData{FieldA: "foo"}, event.ID(id), event.Time(now), event.Aggregate("foobar", uuid.New(), 2)),
			want: false,
		},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			if event.Equal(tt.a, tt.b) != tt.want {
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
	id := uuid.New()
	now := time.Now()
	events := []event.Event{
		event.New("foo", newMockData(), event.ID(id), event.Time(now)),
		event.New("foo", newMockData(), event.ID(id), event.Time(now)),
		event.New("foo", newMockData(), event.ID(id), event.Time(now)),
	}

	if !event.Equal(events...) {
		t.Error(fmt.Errorf("expected events to be equal but they aren't\n%#v", events))
	}

	events = append(events, event.New("bar", newMockData(), event.ID(id), event.Time(now)))

	if event.Equal(events...) {
		t.Error(fmt.Errorf("expected events not to be equal but they are\n%#v", events))
	}
}

func newMockData() mockData {
	return mockData{FieldA: "foo", FieldB: true}
}
