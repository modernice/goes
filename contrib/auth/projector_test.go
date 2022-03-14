package auth_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/repository"
	"github.com/modernice/goes/contrib/auth"
	"github.com/modernice/goes/event/eventbus"
	"github.com/modernice/goes/event/eventstore"
	"github.com/modernice/goes/internal/testutil"
	"github.com/modernice/goes/projection/schedule"
)

func TestProjector(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus := eventbus.New()
	store := eventstore.WithBus(eventstore.New(), bus)
	permissions := auth.InMemoryPermissionRepository()
	repo := repository.New(store)
	actors := auth.NewUUIDActorRepository(repo)
	roles := auth.NewRoleRepository(repo)

	proj := auth.NewPermissionProjector(permissions, bus, store, schedule.Debounce(50*time.Millisecond))

	errs, err := proj.Run(ctx)
	if err != nil {
		t.Fatalf("run projector: %v", err)
	}
	go testutil.PanicOn(errs)

	order := aggregate.Ref{
		Name: "order",
		ID:   uuid.New(),
	}

	// a customer is granted the permission to "view" an "order"
	cus := auth.NewUUIDActor(uuid.New())
	cus.Grant(order, "view")

	// an "admin" role is granted the permission to "view" and "update" the order
	admins := auth.NewRole(uuid.New())
	admins.Identify("admin")
	admins.Grant(order, "view", "update")

	// a user is given the "admin" role
	admin := auth.NewUUIDActor(uuid.New())
	admins.Add(admin.AggregateID())

	// when the actors and role are saved, the projector should be triggered.
	if err := actors.Save(ctx, cus); err != nil {
		t.Fatalf("save customer: %v", err)
	}
	if err := actors.Save(ctx, admin); err != nil {
		t.Fatalf("save admin: %v", err)
	}
	if err := roles.Save(ctx, admins); err != nil {
		t.Fatalf("save %q role: %v", "admin", err)
	}

	<-time.After(200 * time.Millisecond)

	// the customer should have the permission to view the order.
	perms, err := permissions.Fetch(ctx, cus.AggregateID())
	if err != nil {
		t.Fatalf("fetch customer permissions: %v", err)
	}

	if !perms.Allows("view", order) {
		t.Fatalf("customer should have permission to view the order")
	}

	// the admin should have the permission to view and update the order.
	perms, err = permissions.Fetch(ctx, admin.AggregateID())
	if err != nil {
		t.Fatalf("fetch admin permissions: %v", err)
	}

	if !perms.Allows("view", order) {
		t.Fatalf("admin should have permission to view the order")
	}

	if !perms.Allows("update", order) {
		t.Fatalf("admin should have permission to update the order")
	}
}
