package auth_test

import (
	"context"
	"testing"
	"time"

	"github.com/modernice/goes/aggregate/repository"
	"github.com/modernice/goes/contrib/auth"
	"github.com/modernice/goes/event/eventbus"
	"github.com/modernice/goes/event/eventstore"
	"github.com/modernice/goes/internal"
	"github.com/modernice/goes/internal/testutil"
)

func TestLookup(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus := eventbus.New()
	store := eventstore.WithBus(eventstore.New(), bus)
	repo := repository.New(store)
	actors := auth.NewStringActorRepository(repo)

	sid := "foo-id"
	actor := auth.NewStringActor(internal.NewUUID())
	actor.Identify(sid)

	look := auth.NewLookup(store, bus)
	errs, err := look.Run(ctx)
	if err != nil {
		t.Fatalf("run lookup: %v", err)
	}
	go testutil.PanicOn(errs)

	if err := actors.Save(ctx, actor); err != nil {
		t.Fatalf("save actor: %v", err)
	}

	<-time.After(100 * time.Millisecond)

	id, ok := look.Reverse(ctx, auth.ActorAggregate, auth.LookupActor, sid)
	if !ok {
		t.Fatalf("Reverse() should provide the actor id")
	}

	if id != actor.AggregateID() {
		t.Fatalf("Reverse() returned wrong actor id. %s != %s", id, actor.AggregateID())
	}

	id, ok = look.Actor(ctx, sid)
	if !ok {
		t.Fatalf("Actor() should provide the actor id")
	}

	if id != actor.AggregateID() {
		t.Fatalf("Reverse() returned wrong actor id. %s != %s", id, actor.AggregateID())

	}

	// TODO(bounoable): Test lookup of roles.
}
