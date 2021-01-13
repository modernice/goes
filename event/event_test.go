package event_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/event"
	"github.com/stretchr/testify/assert"
)

type mockEvent struct {
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

func newMockData() mockEvent {
	return mockEvent{FieldA: "foo", FieldB: true}
}
