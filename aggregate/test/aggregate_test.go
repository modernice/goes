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
	handled := make(chan event.Of[any, uuid.UUID], 1)
	foo := test.NewFoo(uuid.New(), test.ApplyEventFunc("foo", func(evt event.Of[any, uuid.UUID]) {
		handled <- evt
	}))

	evt := event.New[any](uuid.New(), "bar", eventtest.BarEventData{})
	foo.ApplyEvent(evt)

	select {
	case <-handled:
		t.Fatalf("%q event should not have been handled", evt.Name())
	case <-time.After(10 * time.Millisecond):
	}

	evt = event.New[any](uuid.New(), "foo", eventtest.FooEventData{})
	foo.ApplyEvent(evt)

	select {
	case <-time.After(100 * time.Millisecond):
	case hevt := <-handled:
		if !event.Equal(hevt, evt.Event()) {
			t.Fatalf("received wrong event\n\nwant: %#v\n\ngot: %#v\n\n", evt, hevt)
		}
	}
}

func TestTrackChangeFunc(t *testing.T) {
	aggregateID := uuid.New()
	events := []event.Of[any, uuid.UUID]{
		event.New[any](uuid.New(), "foo", eventtest.FooEventData{}, event.Aggregate(aggregateID, "foo", 1)),
		event.New[any](uuid.New(), "foo", eventtest.FooEventData{}, event.Aggregate(aggregateID, "foo", 2)),
		event.New[any](uuid.New(), "foo", eventtest.FooEventData{}, event.Aggregate(aggregateID, "foo", 3)),
	}

	var tracked bool
	foo := test.NewFoo(
		aggregateID,
		test.TrackChangeFunc(func(changes []event.Of[any, uuid.UUID], track func(...event.Of[any, uuid.UUID])) {
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

func TestCommitFunc(t *testing.T) {
	aggregateID := uuid.New()
	foo := test.NewFoo(aggregateID, test.CommitFunc[uuid.UUID](func(flush func()) {}))
	events := []event.Of[any, uuid.UUID]{
		event.New[any](uuid.New(), "foo", eventtest.FooEventData{}, event.Aggregate(foo.AggregateID(), foo.AggregateName(), 1)),
		event.New[any](uuid.New(), "foo", eventtest.FooEventData{}, event.Aggregate(foo.AggregateID(), foo.AggregateName(), 2)),
		event.New[any](uuid.New(), "foo", eventtest.FooEventData{}, event.Aggregate(foo.AggregateID(), foo.AggregateName(), 3)),
	}
	foo.TrackChange(events...)

	foo.Commit()
	eventtest.AssertEqualEvents(t, events, foo.AggregateChanges())

	foo = test.NewFoo(aggregateID, test.CommitFunc[uuid.UUID](func(flush func()) {
		flush()
	}))
	foo.TrackChange(events...)

	foo.Commit()
	eventtest.AssertEqualEvents(t, foo.AggregateChanges(), nil)
}
