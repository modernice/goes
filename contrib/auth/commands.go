package auth

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/command"
	"github.com/modernice/goes/helper/streams"
)

// Commands
const (
	IdentifyActorCmd   = "goes.contrib.auth.actor.identify"
	IdentifyRoleCmd    = "goes.contrib.auth.role.identify"
	GiveRoleToCmd      = "goes.contrib.auth.role.give"
	RemoveRoleFromCmd  = "goes.contrib.auth.role.remove"
	GrantToActorCmd    = "goes.contib.auth.actor.grant"
	RevokeFromActorCmd = "goes.contib.auth.actor.revoke"
	GrantToRoleCmd     = "goes.contib.auth.role.grant"
	RevokeFromRoleCmd  = "goes.contib.auth.role.revoke"
)

// IdentifyActor returns the command to specify the id of an actor that is not a UUID-Actor.
func IdentifyActor[ID comparable](uid uuid.UUID, id ID) command.Cmd[ID] {
	return command.New(IdentifyActorCmd, id, command.Aggregate(ActorAggregate, uid))
}

// IdentifyRole returns the command to specify the name of the given role.
func IdentifyRole(id uuid.UUID, name string) command.Cmd[string] {
	return command.New(IdentifyRoleCmd, name, command.Aggregate(RoleAggregate, id))
}

// GiveRoleTo returns the command to give the given role to the given actors.
func GiveRoleTo(roleID uuid.UUID, actors ...uuid.UUID) command.Cmd[[]uuid.UUID] {
	return command.New(GiveRoleToCmd, actors, command.Aggregate(RoleAggregate, roleID))
}

// RemoveFoleFrom returns the command to remove the given actors as members from the given role.
func RemoveRoleFrom(roleID uuid.UUID, actors ...uuid.UUID) command.Cmd[[]uuid.UUID] {
	return command.New(RemoveRoleFromCmd, actors, command.Aggregate(RoleAggregate, roleID))
}

type grantActorPayload struct {
	Ref     aggregate.Ref
	Actions []string
}

type revokeActorPayload struct {
	Ref     aggregate.Ref
	Actions []string
}

// GrantToActor returns the command to grant the the given actions to the given actor.
func GrantToActor(actorID uuid.UUID, ref aggregate.Ref, actions ...string) command.Cmd[grantActorPayload] {
	return command.New(GrantToActorCmd, grantActorPayload{Ref: ref, Actions: actions}, command.Aggregate(ActorAggregate, actorID))
}

// RevokeFromActor returns the command to revoke the given actions from the given actor.
func RevokeFromActor(actorID uuid.UUID, ref aggregate.Ref, actions ...string) command.Cmd[revokeActorPayload] {
	return command.New(RevokeFromActorCmd, revokeActorPayload{Ref: ref, Actions: actions}, command.Aggregate(ActorAggregate, actorID))
}

type grantRolePayload struct {
	Ref     aggregate.Ref
	Actions []string
}

type revokeRolePayload struct {
	Ref     aggregate.Ref
	Actions []string
}

// GrantToRole returns the command to grant the the given actions to the given role.
func GrantToRole(roleID uuid.UUID, ref aggregate.Ref, actions ...string) command.Cmd[grantRolePayload] {
	return command.New(GrantToRoleCmd, grantRolePayload{Ref: ref, Actions: actions}, command.Aggregate(RoleAggregate, roleID))
}

// RevokeFromRole returns the command to revoke the the given actions from the given role.
func RevokeFromRole(roleID uuid.UUID, ref aggregate.Ref, actions ...string) command.Cmd[revokeRolePayload] {
	return command.New(RevokeFromRoleCmd, revokeRolePayload{Ref: ref, Actions: actions}, command.Aggregate(RoleAggregate, roleID))
}

// RegisterCommands registers the commands of the auth package into a registry.
func RegisterCommands(r codec.Registerer) {
	codec.Register[any](r, IdentifyActorCmd)
	codec.Register[string](r, IdentifyRoleCmd)
	codec.Register[[]uuid.UUID](r, GiveRoleToCmd)
	codec.Register[[]uuid.UUID](r, RemoveRoleFromCmd)
	codec.Register[grantActorPayload](r, GrantToActorCmd)
	codec.Register[revokeActorPayload](r, RevokeFromActorCmd)
	codec.Register[grantRolePayload](r, GrantToRoleCmd)
	codec.Register[revokeRolePayload](r, RevokeFromRoleCmd)
}

// HandleCommands handles commands until ctx is canceled.
func HandleCommands(
	ctx context.Context,
	bus command.Bus,
	actorRepos ActorRepositories,
	roles RoleRepository,
) (<-chan error, error) {
	identifyActorErrors := command.MustHandle(ctx, bus, IdentifyActorCmd, func(ctx command.Context) error {
		id := ctx.Payload()

		kind, err := actorRepos.ParseKind(id)
		if err != nil {
			return fmt.Errorf("parse kind of %s: %w", id, err)
		}

		repo, err := actorRepos.Repository(kind)
		if err != nil {
			return fmt.Errorf("get %q repository: %w", kind, err)
		}

		return repo.Use(ctx, ctx.AggregateID(), func(a *Actor) error {
			return a.Identify(id)
		})
	})

	identifyRoleErrors := command.MustHandle(ctx, bus, IdentifyRoleCmd, func(ctx command.Ctx[string]) error {
		return roles.Use(ctx, ctx.AggregateID(), func(r *Role) error {
			return r.Identify(ctx.Payload())
		})
	})

	giveRoleErrors := command.MustHandle(ctx, bus, GiveRoleToCmd, func(ctx command.Ctx[[]uuid.UUID]) error {
		return roles.Use(ctx, ctx.AggregateID(), func(r *Role) error {
			return r.Add(ctx.Payload()...)
		})
	})

	removeRoleErrors := command.MustHandle(ctx, bus, RemoveRoleFromCmd, func(ctx command.Ctx[[]uuid.UUID]) error {
		return roles.Use(ctx, ctx.AggregateID(), func(r *Role) error {
			return r.Remove(ctx.Payload()...)
		})
	})

	grantActorErrors := command.MustHandle(ctx, bus, GrantToActorCmd, func(ctx command.Ctx[grantActorPayload]) error {
		load := ctx.Payload()

		actors, err := actorRepos.Repository(UUIDActor)
		if err != nil {
			return fmt.Errorf("get %q repository: %w", UUIDActor, err)
		}

		return actors.Use(ctx, ctx.AggregateID(), func(a *Actor) error {
			return a.Grant(load.Ref, load.Actions...)
		})
	})

	revokeActorErrors := command.MustHandle(ctx, bus, RevokeFromActorCmd, func(ctx command.Ctx[revokeActorPayload]) error {
		load := ctx.Payload()

		actors, err := actorRepos.Repository(UUIDActor)
		if err != nil {
			return fmt.Errorf("get %q repository: %w", UUIDActor, err)
		}

		return actors.Use(ctx, ctx.AggregateID(), func(a *Actor) error {
			return a.Revoke(load.Ref, load.Actions...)
		})
	})

	grantRoleErrors := command.MustHandle(ctx, bus, GrantToRoleCmd, func(ctx command.Ctx[grantRolePayload]) error {
		load := ctx.Payload()
		return roles.Use(ctx, ctx.AggregateID(), func(r *Role) error {
			return r.Grant(load.Ref, load.Actions...)
		})
	})

	revokeRoleErrors := command.MustHandle(ctx, bus, RevokeFromRoleCmd, func(ctx command.Ctx[grantRolePayload]) error {
		load := ctx.Payload()
		return roles.Use(ctx, ctx.AggregateID(), func(r *Role) error {
			return r.Revoke(load.Ref, load.Actions...)
		})
	})

	return streams.FanInAll(
		identifyActorErrors,
		identifyRoleErrors,
		giveRoleErrors,
		removeRoleErrors,
		grantActorErrors,
		revokeActorErrors,
		grantRoleErrors,
		revokeRoleErrors,
	), nil
}
