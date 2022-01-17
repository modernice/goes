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
	b := aggregate.New[any]("foo", id)
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
	wantType := []event.Event[any]{}
	if reflect.TypeOf(changes) != reflect.TypeOf(wantType) {
		t.Errorf("b.Changes should return type %T; got %T", wantType, changes)
	}
	if len(changes) != 0 {
		t.Errorf("b.Changes should return an empty slice")
	}
}

func TestNew_version(t *testing.T) {
	want := 3
	a := aggregate.New[any]("foo", uuid.New(), aggregate.Version[any](want))
	if v := a.AggregateVersion(); v != want {
		t.Fatalf("a.AggregateVersion should return %d; got %d", want, v)
	}
}

func TestBase_TrackChange(t *testing.T) {
	aggregateID := uuid.New()
	b := aggregate.New[any]("foo", aggregateID)
	events := []event.Event[any]{
		event.New[any]("foo", etest.FooEventData{A: "foo"}, event.Aggregate[any](aggregateID, "foo", 1)),
		event.New[any]("foo", etest.FooEventData{A: "foo"}, event.Aggregate[any](aggregateID, "foo", 2)),
		event.New[any]("foo", etest.FooEventData{A: "foo"}, event.Aggregate[any](aggregateID, "foo", 3)),
	}
	b.TrackChange(events...)
	if changes := b.AggregateChanges(); !reflect.DeepEqual(events, changes) {
		t.Fatalf("b.AggregateChanges() returned wrong events\n\nwant: %#v\n\ngot: %#v", events, changes)
	}
}

func TestBase_FlushChanges(t *testing.T) {
	aggregateID := uuid.New()
	b := aggregate.New[any]("foo", aggregateID)
	events := []event.Event[any]{
		event.New[any]("foo", etest.FooEventData{A: "foo"}, event.Aggregate[any](aggregateID, "foo", 1)),
		event.New[any]("foo", etest.FooEventData{A: "foo"}, event.Aggregate[any](aggregateID, "foo", 2)),
		event.New[any]("foo", etest.FooEventData{A: "foo"}, event.Aggregate[any](aggregateID, "foo", 3)),
	}

	b.TrackChange(events...)

	b.Commit()

	if changes := b.AggregateChanges(); len(changes) != 0 {
		t.Fatalf("expected b.AggregateChanges to return an empty slice; got %#v", changes)
	}

	if v := b.AggregateVersion(); v != 3 {
		t.Fatalf("expected b.AggregateVersion to return %d; got %d", 3, v)
	}
}

func TestApplyHistory(t *testing.T) {
	var applied []event.Event[any]
	var flushed bool
	foo := test.NewFoo(
		uuid.New(),
		test.ApplyEventFunc("foo", func(evt event.Event[any]) {
			applied = append(applied, evt)
		}),
		test.CommitFunc(func(flush func()) {
			flush()
			flushed = true
		}),
	)

	events := []event.Event[any]{
		event.New[any]("foo", etest.FooEventData{A: "foo"}, event.Aggregate[any](foo.AggregateID(), foo.AggregateName(), 1)),
		event.New[any]("foo", etest.FooEventData{A: "foo"}, event.Aggregate[any](foo.AggregateID(), foo.AggregateName(), 2)),
		event.New[any]("foo", etest.FooEventData{A: "foo"}, event.Aggregate[any](foo.AggregateID(), foo.AggregateName(), 3)),
	}

	if err := aggregate.ApplyHistory[any](foo, events...); err != nil {
		t.Fatalf("history could not be applied: %v", err)
	}

	if !flushed {
		t.Errorf("aggregate changes weren't flushed")
	}

	etest.AssertEqualEvents(t, events, applied)
}

func TestUncommittedVersion(t *testing.T) {
	a := aggregate.New[any]("foo", uuid.New())

	if v := aggregate.UncommittedVersion[any](a); v != 0 {
		t.Errorf("current aggregate version should be %d; got %d", 0, v)
	}

	evt := event.New[any]("foo", etest.FooEventData{}, event.Aggregate[any](a.AggregateID(), a.AggregateName(), 1))

	a.TrackChange(evt)

	if v := aggregate.UncommittedVersion[any](a); v != 1 {
		t.Errorf("current aggregate version should be %d; got %d", 1, v)
	}

	evt = event.New[any]("foo", etest.FooEventData{}, event.Aggregate[any](a.AggregateID(), a.AggregateName(), 2))

	a.TrackChange(evt)

	if v := aggregate.UncommittedVersion[any](a); v != 2 {
		t.Errorf("current aggregate version should be %d; got %d", 2, v)
	}
}

func TestNextEvent(t *testing.T) {
	a := aggregate.New[any]("foo", uuid.New(), aggregate.Version[any](3))
	data := etest.FooEventData{A: "foo"}
	evt := aggregate.NextEvent[any](a, "bar", data)

	id, name, v := evt.Aggregate()

	if name != a.AggregateName() {
		t.Errorf("Event should have AggregateName %q; is %q", a.AggregateName(), name)
	}

	if id != a.AggregateID() {
		t.Errorf("Event should have AggregateID %s; is %s", a.AggregateID(), id)
	}

	if v != 4 {
		t.Errorf("Event should have AggregateVersion 4; is %d", v)
	}

	if evt.Data() != data {
		t.Errorf("Event should have data %v; got %v", data, evt.Data())
	}
}

func TestHasChange(t *testing.T) {
	a := aggregate.New[any]("foo", uuid.New())
	aggregate.NextEvent[any](a, "foo", etest.FooEventData{})
	aggregate.NextEvent[any](a, "bar", etest.BarEventData{})
	aggregate.NextEvent[any](a, "baz", etest.BazEventData{})

	wantChanges := []string{"foo", "bar", "baz"}
	for _, name := range wantChanges {
		if !aggregate.HasChange[any](a, name) {
			t.Fatalf("Aggregate should have %q change", name)
		}
	}

	if aggregate.HasChange[any](a, "foobar") {
		t.Fatalf("Aggregate shouldn't have %q change", "foobar")
	}
}
