package auth

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/helper/streams"
	"github.com/modernice/goes/projection"
	"github.com/modernice/goes/projection/schedule"
)

// Granter subscribes to user-provided events to trigger permission changes.
// Granter can be used to automatically grant or revoke permissions to and from
// actors and roles when a specified event is published over the underlying
// event bus.
//
// Granter applies permission changes retrospectively for past events on
// startup to ensure that new permissions are applied to existing actors and
// roles.
type Granter struct {
	client   CommandClient
	lookup   Lookup
	schedule *schedule.Continuous
	once     sync.Once
	ready    chan struct{}
}

// TargetedGranter provides grant and revoke methods for the actions on a
// specific aggregate. The provided GrantToXXX() and RevokeFromXXX() methods
// grant the given actor or role permission to perform the given actions on the
// aggregate referenced by Target().
//
// TargetedGranter is passed to PermissionGranterEvent implementations by a
// *Granter when an event with such data is published over the *Granter's underlying
// event bus.
type TargetedGranter interface {
	// Context is the context of the underlying *Granter.
	Context() context.Context

	// Target returns the permission target for the granted and revoked permissions.
	Target() aggregate.Ref

	// Lookup returns the lookup that can be used to resolve actor and role ids.
	Lookup() Lookup

	// GrantToActor grants the given actor the permission to perform the given
	// actions on the aggregate referenced by Target().
	GrantToActor(ctx context.Context, actorID uuid.UUID, actions ...string) error

	// RevokeFromActor revokes from the given actor the permission to perform
	// the given actions on the aggregate referenced by Target().
	RevokeFromActor(ctx context.Context, actorID uuid.UUID, actions ...string) error

	// GrantToRole grants the given role the permission to perform the given
	// actions on the aggregate referenced by Target().
	GrantToRole(ctx context.Context, roleID uuid.UUID, actions ...string) error

	// RevokeFromRole revokes from the given role the permission to perform
	// the given actions on the aggregate referenced by Target().
	RevokeFromRole(ctx context.Context, roleID uuid.UUID, actions ...string) error
}

// PermissionGranterEvent must be implemented by event data to be used within a *Granter.
// Event data that implements this interface can grant and revoke permissions to
// and from actors and roles. When such an event is published over the event bus,
// the running *Granter calls the event data's GrantPermissions() method with a
// TargetedGranter. The aggregate of the event is used as the permission target.
type PermissionGranterEvent interface {
	// GrantPermissions is called by *Granter when the event that implements
	// this interface is published.
	GrantPermissions(TargetedGranter) error
}

// NewGranter returns a new permission granter background task.
//
//	var events []string
//	var actors auth.ActorRepositories
//	var roles auth.RoleRepository
//	var lookup *auth.Lookup
//	var bus event.Bus
//
//	g := auth.NewGranter(events, actors, roles, lookup, bus)
//	errs, err := g.Run(context.TODO())
func NewGranter(
	events []string,
	client CommandClient,
	lookup Lookup,
	bus event.Bus,
	store event.Store,
) *Granter {
	return &Granter{
		client:   client,
		lookup:   lookup,
		schedule: schedule.Continuously(bus, store, events),
	}
}

// Ready returns a channel that blocks until the granter applied a projection
// job for the first time. Waiting for <-g.Ready() ensures that the permissions
// of all actors are up-to-date. Ready should not be called before g.Run()
// is called, otherwise it will return a nil-channel that blocks forever.
// Ready must not be called before g.Run() returns, to avoid race conditions.
func (g *Granter) Ready() <-chan struct{} {
	return g.ready
}

// Run runs the permission granter until ctx is canceled.
func (g *Granter) Run(ctx context.Context) (<-chan error, error) {
	g.ready = make(chan struct{})

	errs, err := g.schedule.Subscribe(ctx, g.applyJob)
	if err != nil {
		return nil, fmt.Errorf("subscribe to projection schedule: %w", err)
	}

	go g.schedule.Trigger(ctx)

	return errs, nil
}

func (g *Granter) applyJob(ctx projection.Job) error {
	defer g.once.Do(func() { close(g.ready) })

	events, errs, err := ctx.Events(ctx)
	if err != nil {
		return fmt.Errorf("get events from job: %w", err)
	}

	return streams.Walk(ctx, func(evt event.Event) error {
		if err := g.applyEvent(ctx, evt); err != nil {
			return fmt.Errorf("apply %q event: %w", evt.Name(), err)
		}
		return nil
	}, events, errs)
}

func (g *Granter) applyEvent(ctx context.Context, evt event.Event) error {
	pge, ok := evt.Data().(PermissionGranterEvent)
	if !ok {
		return fmt.Errorf("%q event does not implement PermissionGranterEvent", evt.Name())
	}

	id, name, _ := evt.Aggregate()

	granter := targetedGranter{
		ctx:    ctx,
		client: g.client,
		lookup: g.lookup,
		target: aggregate.Ref{
			Name: name,
			ID:   id,
		},
	}

	if err := pge.GrantPermissions(granter); err != nil {
		return fmt.Errorf("handle %q event: %w", evt.Name(), err)
	}

	return nil
}

type targetedGranter struct {
	ctx    context.Context
	client CommandClient
	lookup Lookup
	target aggregate.Ref
}

func (tg targetedGranter) Context() context.Context { return tg.ctx }

func (tg targetedGranter) Target() aggregate.Ref { return tg.target }

func (tg targetedGranter) Lookup() Lookup { return tg.lookup }

func (tg targetedGranter) GrantToActor(ctx context.Context, actorID uuid.UUID, actions ...string) error {
	return tg.client.GrantToActor(ctx, actorID, tg.target, actions...)
}

func (tg targetedGranter) GrantToRole(ctx context.Context, roleID uuid.UUID, actions ...string) error {
	return tg.client.GrantToRole(ctx, roleID, tg.target, actions...)
}

func (tg targetedGranter) RevokeFromActor(ctx context.Context, actorID uuid.UUID, actions ...string) error {
	return tg.client.RevokeFromActor(ctx, actorID, tg.target, actions...)
}

func (tg targetedGranter) RevokeFromRole(ctx context.Context, roleID uuid.UUID, actions ...string) error {
	return tg.client.RevokeFromRole(ctx, roleID, tg.target, actions...)
}
