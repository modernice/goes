package event_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/event"
	"github.com/stretchr/testify/assert"
)

type mockData struct {
	FieldA string
	FieldB bool
}

func TestNewEvent(t *testing.T) {
	data := newMockData()
	evt := event.New("foo", data)

	assert.IsType(t, uuid.UUID{}, evt.ID())
	assert.NotEqual(t, uuid.Nil, evt.ID())
	assert.Equal(t, "foo", evt.Name())
	assert.Equal(t, data, evt.Data())
	assert.IsType(t, time.Time{}, evt.Time())
	assert.InDelta(t, time.Now().Unix(), evt.Time().Unix(), 1)
	assert.Equal(t, "", evt.AggregateName())
	assert.Equal(t, uuid.Nil, evt.AggregateID())
	assert.Equal(t, 0, evt.AggregateVersion())
}

func TestNewEvent_time(t *testing.T) {
	ts := time.Now().Add(time.Hour)
	evt := event.New("foo", newMockData(), event.Time(ts))
	assert.Equal(t, ts, evt.Time())
}

func TestNewEvent_aggregate(t *testing.T) {
	aname := "bar"
	aid := uuid.New()
	v := 3

	evt := event.New("foo", newMockData(), event.Aggregate(aname, aid, v))
	assert.Equal(t, "bar", evt.AggregateName())
	assert.Equal(t, aid, evt.AggregateID())
	assert.Equal(t, v, evt.AggregateVersion())
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
					t.Error(fmt.Errorf("expected events to be equal but they aren't\nevent a: %#v\n\nevent b: %#v", tt.a, tt.b))
					return
				}
				t.Error(fmt.Errorf("expected events not to be equal but they are\nevent a: %#v\n\nevent b: %#v", tt.a, tt.b))
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
