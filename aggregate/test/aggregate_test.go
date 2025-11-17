package test_test

import (
	"testing"
	"time"

	"github.com/modernice/goes/aggregate/test"
	"github.com/modernice/goes/event"
	eventtest "github.com/modernice/goes/event/test"
	"github.com/modernice/goes/internal"
)

func TestApplyEventFunc(t *testing.T) {
	handled := make(chan event.Event, 1)
	foo := test.NewFoo(internal.NewUUID(), test.ApplyEventFunc("foo", func(evt event.Event) {
		handled <- evt
	}))

	evt := event.New[any]("bar", eventtest.BarEventData{})
	foo.ApplyEvent(evt)

	select {
	case <-handled:
		t.Fatalf("%q event should not have been handled", evt.Name())
	case <-time.After(10 * time.Millisecond):
	}

	evt = event.New[any]("foo", eventtest.FooEventData{})
	foo.ApplyEvent(evt)

	select {
	case <-time.After(100 * time.Millisecond):
	case hevt := <-handled:
		if !event.Equal(hevt, evt.Event()) {
			t.Fatalf("received wrong event\n\nwant: %#v\n\ngot: %#v\n\n", evt, hevt)
		}
	}
}

func TestRecordChangeFunc(t *testing.T) {
	aggregateID := internal.NewUUID()
	events := []event.Event{
		event.New[any]("foo", eventtest.FooEventData{}, event.Aggregate(aggregateID, "foo", 1)),
		event.New[any]("foo", eventtest.FooEventData{}, event.Aggregate(aggregateID, "foo", 2)),
		event.New[any]("foo", eventtest.FooEventData{}, event.Aggregate(aggregateID, "foo", 3)),
	}

	var tracked bool
	foo := test.NewFoo(
		aggregateID,
		test.RecordChangeFunc(func(changes []event.Event, track func(...event.Event)) {
			tracked = true
			track(changes...)
		}),
	)

	foo.RecordChange(events...)

	if !tracked {
		t.Errorf("changes were not tracked")
	}

	eventtest.AssertEqualEvents(t, events, foo.AggregateChanges())
}

func TestCommitFunc(t *testing.T) {
	aggregateID := internal.NewUUID()
	foo := test.NewFoo(aggregateID, test.CommitFunc(func(flush func()) {}))
	events := []event.Event{
		event.New[any]("foo", eventtest.FooEventData{}, event.Aggregate(foo.AggregateID(), foo.AggregateName(), 1)),
		event.New[any]("foo", eventtest.FooEventData{}, event.Aggregate(foo.AggregateID(), foo.AggregateName(), 2)),
		event.New[any]("foo", eventtest.FooEventData{}, event.Aggregate(foo.AggregateID(), foo.AggregateName(), 3)),
	}
	foo.RecordChange(events...)

	foo.Commit()
	eventtest.AssertEqualEvents(t, events, foo.AggregateChanges())

	foo = test.NewFoo(aggregateID, test.CommitFunc(func(flush func()) {
		flush()
	}))
	foo.RecordChange(events...)

	foo.Commit()
	eventtest.AssertEqualEvents(t, foo.AggregateChanges(), nil)
}
