package test_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate/test"
	"github.com/modernice/goes/event"
	eventtest "github.com/modernice/goes/event/test"
)

func TestApplyEventFunc(t *testing.T) {
	handled := make(chan event.Event, 1)
	foo := test.NewFoo(uuid.New(), test.ApplyEventFunc("foo", func(evt event.Event) {
		handled <- evt
	}))

	evt := event.New("bar", eventtest.BarEventData{})
	foo.ApplyEvent(evt)

	select {
	case <-handled:
		t.Fatalf("%q event should not have been handled", evt.Name())
	case <-time.After(10 * time.Millisecond):
	}

	evt = event.New("foo", eventtest.FooEventData{})
	foo.ApplyEvent(evt)

	select {
	case <-time.After(100 * time.Millisecond):
	case hevt := <-handled:
		if !event.Equal(hevt, evt) {
			t.Fatalf("received wrong event\n\nwant: %#v\n\ngot: %#v\n\n", evt, hevt)
		}
	}
}

func TestTrackChangeFunc(t *testing.T) {
	aggregateID := uuid.New()
	events := []event.Event{
		event.New("foo", eventtest.FooEventData{}, event.Aggregate("foo", aggregateID, 1)),
		event.New("foo", eventtest.FooEventData{}, event.Aggregate("foo", aggregateID, 2)),
		event.New("foo", eventtest.FooEventData{}, event.Aggregate("foo", aggregateID, 3)),
	}

	var tracked bool
	foo := test.NewFoo(
		aggregateID,
		test.TrackChangeFunc(func(changes []event.Event, track func(...event.Event)) {
			tracked = true
			track(changes...)
		}),
	)

	foo.TrackChange(events...)

	if !tracked {
		t.Errorf("changes were not tracked")
	}

	eventtest.AssertEqualEvents(t, events, foo.AggregateChanges())
}

func TestFlushChangesFunc(t *testing.T) {
	aggregateID := uuid.New()
	foo := test.NewFoo(aggregateID, test.FlushChangesFunc(func(flush func()) {}))
	events := []event.Event{
		event.New("foo", eventtest.FooEventData{}, event.Aggregate(foo.AggregateName(), foo.AggregateID(), 1)),
		event.New("foo", eventtest.FooEventData{}, event.Aggregate(foo.AggregateName(), foo.AggregateID(), 2)),
		event.New("foo", eventtest.FooEventData{}, event.Aggregate(foo.AggregateName(), foo.AggregateID(), 3)),
	}
	foo.TrackChange(events...)

	foo.FlushChanges()
	eventtest.AssertEqualEvents(t, events, foo.AggregateChanges())

	foo = test.NewFoo(aggregateID, test.FlushChangesFunc(func(flush func()) {
		flush()
	}))
	foo.TrackChange(events...)

	foo.FlushChanges()
	eventtest.AssertEqualEvents(t, foo.AggregateChanges(), nil)
}
