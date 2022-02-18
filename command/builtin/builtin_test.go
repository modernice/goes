package builtin_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/repository"
	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/command/builtin"
	"github.com/modernice/goes/command/cmdbus"
	"github.com/modernice/goes/command/cmdbus/dispatch"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventbus"
	"github.com/modernice/goes/event/eventstore"
	"github.com/modernice/goes/event/test"
	"github.com/modernice/goes/helper/pick"
	"github.com/modernice/goes/helper/streams"
)

func TestDeleteAggregate(t *testing.T) {
	aggregateName := "foo"
	aggregateID := uuid.New()

	cmd := builtin.DeleteAggregate(aggregateName, aggregateID)

	if cmd.Name() != "goes.command.aggregate.delete" {
		t.Fatalf("Name() should return %q; got %q", "goes.command.aggregate.delete", cmd.Name())
	}

	id, name := cmd.Aggregate()

	if name != aggregateName {
		t.Fatalf("AggregateName() should return %q; got %q", aggregateName, name)
	}

	if id != aggregateID {
		t.Fatalf("AggregateID() should return %q; got %q", aggregateID, id)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ebus := eventbus.New[uuid.UUID]()
	estore := eventstore.WithBus(eventstore.New[uuid.UUID](), ebus)
	repo := repository.New(estore)
	reg := codec.New()
	builtin.RegisterCommands(reg)

	bus := cmdbus.New(uuid.New, reg, ebus)

	go panicOn(builtin.MustHandle[uuid.UUID](ctx, uuid.New, bus, repo, builtin.PublishEvents(ebus, nil)))

	foo := newMockAggregate(aggregateID)
	newMockEvent(foo, 2)
	newMockEvent(foo, 4)
	newMockEvent(foo, 8)

	if foo.Foo != 14 {
		t.Fatalf("Foo should be %d; is %d", 14, foo.Foo)
	}

	if aggregate.UncommittedVersion[uuid.UUID](foo) != 3 {
		t.Fatalf("AggregateVersion() should return %d; got %d", 3, foo.AggregateVersion())
	}

	if err := repo.Save(ctx, foo); err != nil {
		t.Fatalf("save aggregate: %v", err)
	}

	// Check that the fetched aggregate has the correct state:

	foo = newMockAggregate(foo.ID)
	if err := repo.Fetch(ctx, foo); err != nil {
		t.Fatalf("fetch aggregate: %v", err)
	}

	if foo.AggregateVersion() != 3 {
		t.Fatalf("AggregateVersion() should return %d; got %d", 3, foo.AggregateVersion())
	}

	if foo.Foo != 14 {
		t.Fatalf("Foo should be %d; is %d", 14, foo.Foo)
	}

	awaitCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	str, errs := event.Must(eventbus.Await[any](awaitCtx, ebus, builtin.AggregateDeleted))

	if err := bus.Dispatch(ctx, cmd.Any(), dispatch.Sync()); err != nil {
		t.Fatalf("dispatch command: %v", err)
	}

	// A "goes.command.aggregate.deleted" event should be published
	evt, err := streams.Await(ctx, str, errs)
	if err != nil {
		t.Fatalf("await event: %v", err)
	}

	if evt.Name() != builtin.AggregateDeleted {
		t.Fatalf("Event name should b %q; is %q", builtin.AggregateDeleted, evt.Name())
	}

	data, ok := evt.Data().(builtin.AggregateDeletedData)
	if !ok {
		t.Fatalf("Data() should return type %T; got %T", data, evt.Data())
	}

	if pick.AggregateName[uuid.UUID](evt) != aggregateName {
		t.Fatalf("evt.AggregateName() should be %q; is %q", aggregateName, pick.AggregateName[uuid.UUID](evt))
	}

	if pick.AggregateID[uuid.UUID](evt) != aggregateID {
		t.Fatalf("evt.AggregateID() should return %q; is %q", aggregateID, pick.AggregateID[uuid.UUID](evt))
	}

	if pick.AggregateVersion[uuid.UUID](evt) != 0 {
		t.Fatalf("evt.AggregateVersion() should return 0; got %v", pick.AggregateVersion[uuid.UUID](evt))
	}

	if data.Version != 3 {
		t.Fatalf("Version should be %v; is %v", 3, data.Version)
	}

	// Deleted aggregate should have zero-state when fetched:

	foo = newMockAggregate(foo.ID)
	if err := repo.Fetch(ctx, foo); err != nil {
		t.Fatalf("fetch aggregate: %v", err)
	}

	if foo.AggregateVersion() != 0 {
		t.Fatalf("AggregateVersion() should return 0 for deleted aggregate; got %d", foo.AggregateVersion())
	}

	if foo.Foo != 0 {
		t.Fatalf("Foo should be 0; is %d", foo.Foo)
	}
}

func panicOn(errs <-chan error) {
	for err := range errs {
		panic(err)
	}
}

type mockAggregate struct {
	*aggregate.Base[uuid.UUID]

	Foo int
}

func newMockAggregate(id uuid.UUID) *mockAggregate {
	return &mockAggregate{
		Base: aggregate.New("foo", id),
	}
}

func (ma *mockAggregate) ApplyEvent(evt event.Of[any, uuid.UUID]) {
	data := evt.Data().(test.FoobarEventData)
	ma.Foo += data.A
}

func newMockEvent(a aggregate.AggregateOf[uuid.UUID], foo int) event.E[test.FoobarEventData, uuid.UUID] {
	return aggregate.NextEvent(a, uuid.New(), "foobar", test.FoobarEventData{A: foo})
}
