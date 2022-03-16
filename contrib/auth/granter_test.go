package auth_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/repository"
	"github.com/modernice/goes/contrib/auth"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventbus"
	"github.com/modernice/goes/event/eventstore"
	"github.com/modernice/goes/internal/testutil"
)

func TestGranter_event_actor(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	gt := NewGrantTest(t)

	sactors, _ := gt.actors.Repository(auth.StringActor)

	actor := auth.NewStringActor(uuid.New())
	actor.Identify("foo")

	if err := sactors.Save(ctx, actor); err != nil {
		t.Fatalf("save actor: %v", err)
	}

	ref := aggregate.Ref{
		Name: "acted-on",
		ID:   uuid.New(),
	}
	actions := []string{"foo", "bar", "baz"}

	gt.Run(ctx)

	evt := event.New("granted", granterEvent{
		actorID: "foo",
		actions: actions,
	}, event.Aggregate(ref.ID, ref.Name, 1))

	if err := gt.bus.Publish(ctx, evt.Any()); err != nil {
		t.Fatalf("publish event: %v", err)
	}

	gt.ExpectPermissions(ctx, actor.AggregateID(), ref, actions)
}

func TestGranter_event_role(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	gt := NewGrantTest(t)
	gt.Run(ctx)

	uidactors, _ := gt.actors.Repository(auth.UUIDActor)

	actor := auth.NewUUIDActor(uuid.New())

	if err := uidactors.Save(ctx, actor); err != nil {
		t.Fatalf("save actor: %v", err)
	}

	role := auth.NewRole(uuid.New())
	role.Identify("admin")
	role.Add(actor.AggregateID())

	if err := gt.roles.Save(ctx, role); err != nil {
		t.Fatalf("save role: %v", err)
	}

	// wait for lookup update
	<-time.After(100 * time.Millisecond)

	ref := aggregate.Ref{
		Name: "acted-on",
		ID:   uuid.New(),
	}
	actions := []string{"foo", "bar", "baz"}

	evt := event.New("granted", granterEvent{
		roleName: "admin",
		actions:  actions,
	}, event.Aggregate(ref.ID, ref.Name, 1))

	if err := gt.bus.Publish(ctx, evt.Any()); err != nil {
		t.Fatalf("publish event: %v", err)
	}

	gt.ExpectPermissions(ctx, actor.AggregateID(), ref, actions)
}

func TestStartupGrant(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	gt := NewGrantTest(t)

	actor := auth.NewStringActor(uuid.New())
	actor.Identify("foo")

	sactors, _ := gt.actors.Repository(auth.StringActor)
	if err := sactors.Save(ctx, actor); err != nil {
		t.Fatalf("save actor: %v", err)
	}

	ref := aggregate.Ref{
		Name: "acted-on",
		ID:   uuid.New(),
	}
	actions := []string{"foo", "bar", "baz"}

	evt := event.New("granted", granterEvent{
		actorID: "foo",
		actions: actions,
	}, event.Aggregate(ref.ID, ref.Name, 1))

	if err := gt.store.Insert(ctx, evt.Any()); err != nil {
		t.Fatalf("insert event: %v", err)
	}

	gt.Run(ctx)
	gt.ExpectPermissions(ctx, actor.AggregateID(), ref, actions)
}

type GrantTest struct {
	GrantTestOptions

	t         *testing.T
	actors    auth.ActorRepositories
	roles     auth.RoleRepository
	bus       event.Bus
	store     event.Store
	perms     auth.PermissionRepository
	projector *auth.PermissionProjector
	lookup    *auth.LookupTable
	granter   *auth.Granter
}

type GrantTestOptions struct {
	decorateBus bool
}

func NewGrantTest(t *testing.T) *GrantTest {
	bus := eventbus.New()
	store := eventstore.WithBus(eventstore.New(), bus)

	look := auth.NewLookup(store, bus)
	repo := repository.New(store)
	actors := auth.NewActorRepositories(repo, nil)
	roles := auth.NewRoleRepository(repo)
	permissions := auth.InMemoryPermissionRepository()
	projector := auth.NewPermissionProjector(permissions, roles, bus, store)

	return &GrantTest{
		t:         t,
		actors:    actors,
		roles:     roles,
		bus:       bus,
		store:     store,
		perms:     permissions,
		projector: projector,
		lookup:    look,
	}
}

func (gt *GrantTest) Run(ctx context.Context) {
	errs, err := gt.lookup.Run(ctx)
	if err != nil {
		gt.t.Fatalf("run lookup: %v", err)
	}
	go testutil.PanicOn(errs)

	if errs, err = gt.projector.Run(ctx); err != nil {
		gt.t.Fatalf("run permission projector: %v", err)
	}
	go testutil.PanicOn(errs)

	gt.granter = auth.NewGranter([]string{"granted"}, gt.actors, gt.roles, gt.lookup, gt.bus, gt.store)
	if errs, err = gt.granter.Run(ctx); err != nil {
		gt.t.Fatalf("run granter: %v", err)
	}
	go testutil.PanicOn(errs)

	<-gt.granter.Ready()
}

func (gt *GrantTest) ExpectPermissions(ctx context.Context, actorID uuid.UUID, ref aggregate.Ref, actions []string) {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	timeout := time.NewTimer(3 * time.Second)
	defer timeout.Stop()

L:
	for {
		var allowed []string
		select {
		case <-timeout.C:
			gt.t.Fatalf("actor should be allowed to perform %q actions on %s; allowed=%v", actions, ref, allowed)
		case <-ticker.C:
			perms, err := gt.perms.Fetch(ctx, actorID)
			if err != nil {
				gt.t.Fatalf("fetch actor permissions: %v", err)
			}

			for _, action := range actions {
				if !perms.Allows(action, ref) {
					continue L
				}
				allowed = append(allowed, action)
			}

			return
		}
	}
}

type granterEvent struct {
	actorID  string
	roleName string
	actions  []string
}

func (evt granterEvent) GrantPermissions(g auth.TargetedGranter) error {
	if evt.actorID != "" {
		if actorID, ok := g.Lookup().Actor(g.Context(), evt.actorID); ok {
			if err := g.GrantToActor(g.Context(), actorID, evt.actions...); err != nil {
				return fmt.Errorf("grant to actor: %w", err)
			}
		}
	}

	if evt.roleName != "" {
		if roleID, ok := g.Lookup().Role(g.Context(), evt.roleName); ok {
			if err := g.GrantToRole(g.Context(), roleID, evt.actions...); err != nil {
				return fmt.Errorf("grant to role: %w", err)
			}
		}
	}

	return nil
}
