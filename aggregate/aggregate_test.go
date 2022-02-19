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
	wantType := []event.Of[any, uuid.UUID]{}
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
	events := []event.Of[any, uuid.UUID]{
		event.New(uuid.New(), "foo", etest.FooEventData{A: "foo"}, event.Aggregate(aggregateID, "foo", 1)).Any(),
		event.New(uuid.New(), "foo", etest.FooEventData{A: "foo"}, event.Aggregate(aggregateID, "foo", 2)).Any(),
		event.New(uuid.New(), "foo", etest.FooEventData{A: "foo"}, event.Aggregate(aggregateID, "foo", 3)).Any(),
	}
	b.TrackChange(events...)
	if changes := b.AggregateChanges(); !reflect.DeepEqual(events, changes) {
		t.Fatalf("b.AggregateChanges() returned wrong events\n\nwant: %#v\n\ngot: %#v", events, changes)
	}
}

func TestBase_FlushChanges(t *testing.T) {
	aggregateID := uuid.New()
	b := aggregate.New("foo", aggregateID)
	events := []event.Of[any, uuid.UUID]{
		event.New(uuid.New(), "foo", etest.FooEventData{A: "foo"}, event.Aggregate(aggregateID, "foo", 1)).Any(),
		event.New(uuid.New(), "foo", etest.FooEventData{A: "foo"}, event.Aggregate(aggregateID, "foo", 2)).Any(),
		event.New(uuid.New(), "foo", etest.FooEventData{A: "foo"}, event.Aggregate(aggregateID, "foo", 3)).Any(),
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
	var applied []event.Of[any, uuid.UUID]
	var flushed bool
	foo := test.NewFoo(
		uuid.New(),
		test.ApplyEventFunc("foo", func(evt event.Of[any, uuid.UUID]) {
			applied = append(applied, evt)
		}),
		test.CommitFunc[uuid.UUID](func(flush func()) {
			flush()
			flushed = true
		}),
	)

	events := []event.Of[any, uuid.UUID]{
		event.New(uuid.New(), "foo", etest.FooEventData{A: "foo"}, event.Aggregate(foo.AggregateID(), foo.AggregateName(), 1)).Any(),
		event.New(uuid.New(), "foo", etest.FooEventData{A: "foo"}, event.Aggregate(foo.AggregateID(), foo.AggregateName(), 2)).Any(),
		event.New(uuid.New(), "foo", etest.FooEventData{A: "foo"}, event.Aggregate(foo.AggregateID(), foo.AggregateName(), 3)).Any(),
	}

	if err := aggregate.ApplyHistory[any, uuid.UUID](foo, events); err != nil {
		t.Fatalf("history could not be applied: %v", err)
	}

	if !flushed {
		t.Errorf("aggregate changes weren't flushed")
	}

	etest.AssertEqualEvents(t, events, applied)
}

func TestUncommittedVersion(t *testing.T) {
	a := aggregate.New("foo", uuid.New())

	if v := aggregate.UncommittedVersion[uuid.UUID](a); v != 0 {
		t.Errorf("current aggregate version should be %d; got %d", 0, v)
	}

	evt := event.New(uuid.New(), "foo", etest.FooEventData{}, event.Aggregate(a.AggregateID(), a.AggregateName(), 1)).Any()

	a.TrackChange(evt)

	if v := aggregate.UncommittedVersion[uuid.UUID](a); v != 1 {
		t.Errorf("current aggregate version should be %d; got %d", 1, v)
	}

	evt = event.New(uuid.New(), "foo", etest.FooEventData{}, event.Aggregate(a.AggregateID(), a.AggregateName(), 2)).Any()

	a.TrackChange(evt)

	if v := aggregate.UncommittedVersion[uuid.UUID](a); v != 2 {
		t.Errorf("current aggregate version should be %d; got %d", 2, v)
	}
}

func TestNextEvent(t *testing.T) {
	a := aggregate.New("foo", uuid.New(), aggregate.Version(3))
	data := etest.FooEventData{A: "foo"}
	evt := aggregate.NextEvent[uuid.UUID](a, uuid.New(), "bar", data)

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
	a := aggregate.New("foo", uuid.New())
	aggregate.NextEvent[uuid.UUID](a, uuid.New(), "foo", etest.FooEventData{})
	aggregate.NextEvent[uuid.UUID](a, uuid.New(), "bar", etest.BarEventData{})
	aggregate.NextEvent[uuid.UUID](a, uuid.New(), "baz", etest.BazEventData{})

	wantChanges := []string{"foo", "bar", "baz"}
	for _, name := range wantChanges {
		if !aggregate.HasChange[uuid.UUID](a, name) {
			t.Fatalf("Aggregate should have %q change", name)
		}
	}

	if aggregate.HasChange[uuid.UUID](a, "foobar") {
		t.Fatalf("Aggregate shouldn't have %q change", "foobar")
	}
}
