package aggregate_test

import (
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/test"
	"github.com/modernice/goes/event"
	etest "github.com/modernice/goes/event/test"
)

func TestNew(t *testing.T) {
	id := uuid.New()
	b := aggregate.New("foo", id)
	if b.AggregateID() != id {
		t.Errorf("b.ID should return %v; got %v", id, b.AggregateID())
	}
	if b.AggregateName() != "foo" {
		t.Errorf("b.Name should return %q; got %q", "foo", b.AggregateName())
	}
	if b.AggregateVersion() != 0 {
		t.Errorf("b.Version should return %v; got %v", 0, b.AggregateVersion())
	}
	changes := b.AggregateChanges()
	wantType := []event.Event{}
	if reflect.TypeOf(changes) != reflect.TypeOf(wantType) {
		t.Errorf("b.Changes should return type %T; got %T", wantType, changes)
	}
	if len(changes) != 0 {
		t.Errorf("b.Changes should return an empty slice")
	}
}

func TestNew_version(t *testing.T) {
	want := 3
	a := aggregate.New("foo", uuid.New(), aggregate.Version(want))
	if v := a.AggregateVersion(); v != want {
		t.Fatalf("a.AggregateVersion should return %d; got %d", want, v)
	}
}

func TestBase_TrackChange(t *testing.T) {
	aggregateID := uuid.New()
	b := aggregate.New("foo", aggregateID)
	events := []event.Event{
		event.New("foo", etest.FooEventData{A: "foo"}, event.Aggregate("foo", aggregateID, 1)),
		event.New("foo", etest.FooEventData{A: "foo"}, event.Aggregate("foo", aggregateID, 2)),
		event.New("foo", etest.FooEventData{A: "foo"}, event.Aggregate("foo", aggregateID, 3)),
	}
	if err := b.TrackChange(events...); err != nil {
		t.Fatalf("expected b.TrackChange to succeed; got %#v", err)
	}
	if changes := b.AggregateChanges(); !reflect.DeepEqual(events, changes) {
		t.Fatalf("b.AggregateChanges() returned wrong events\n\nwant: %#v\n\ngot: %#v", events, changes)
	}
}

func TestBase_FlushChanges(t *testing.T) {
	aggregateID := uuid.New()
	b := aggregate.New("foo", aggregateID)
	events := []event.Event{
		event.New("foo", etest.FooEventData{A: "foo"}, event.Aggregate("foo", aggregateID, 1)),
		event.New("foo", etest.FooEventData{A: "foo"}, event.Aggregate("foo", aggregateID, 2)),
		event.New("foo", etest.FooEventData{A: "foo"}, event.Aggregate("foo", aggregateID, 3)),
	}

	if err := b.TrackChange(events...); err != nil {
		t.Fatalf("expected b.TrackChange not to return an error; got %#v", err)
	}

	b.FlushChanges()

	if changes := b.AggregateChanges(); len(changes) != 0 {
		t.Fatalf("expected b.AggregateChanges to return an empty slice; got %#v", changes)
	}

	if v := b.AggregateVersion(); v != 3 {
		t.Fatalf("expected b.AggregateVersion to return %d; got %d", 3, v)
	}
}

func TestApplyHistory(t *testing.T) {
	var applied []event.Event
	var flushed bool
	foo := test.NewFoo(
		uuid.New(),
		test.ApplyEventFunc("foo", func(evt event.Event) {
			applied = append(applied, evt)
		}),
		test.FlushChangesFunc(func(flush func()) {
			flush()
			flushed = true
		}),
	)

	events := []event.Event{
		event.New("foo", etest.FooEventData{A: "foo"}, event.Aggregate(foo.AggregateName(), foo.AggregateID(), 1)),
		event.New("foo", etest.FooEventData{A: "foo"}, event.Aggregate(foo.AggregateName(), foo.AggregateID(), 2)),
		event.New("foo", etest.FooEventData{A: "foo"}, event.Aggregate(foo.AggregateName(), foo.AggregateID(), 3)),
	}

	if err := aggregate.ApplyHistory(foo, events...); err != nil {
		t.Fatalf("history could not be applied: %v", err)
	}

	if !flushed {
		t.Errorf("aggregate changes weren't flushed")
	}

	etest.AssertEqualEvents(t, events, applied)
}

func TestCurrentVersion(t *testing.T) {
	a := aggregate.New("foo", uuid.New())

	if v := aggregate.CurrentVersion(a); v != 0 {
		t.Errorf("current aggregate version should be %d; got %d", 0, v)
	}

	evt := event.New("foo", etest.FooEventData{}, event.Aggregate(a.AggregateName(), a.AggregateID(), 1))

	if err := a.TrackChange(evt); err != nil {
		t.Errorf("failed to track change: %v", err)
	}

	if v := aggregate.CurrentVersion(a); v != 1 {
		t.Errorf("current aggregate version should be %d; got %d", 1, v)
	}

	evt = event.New("foo", etest.FooEventData{}, event.Aggregate(a.AggregateName(), a.AggregateID(), 2))

	if err := a.TrackChange(evt); err != nil {
		t.Errorf("failed to track change: %v", err)
	}

	if v := aggregate.CurrentVersion(a); v != 2 {
		t.Errorf("current aggregate version should be %d; got %d", 2, v)
	}
}
