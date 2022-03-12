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

type grantRevokePayload struct {
	Ref     aggregate.Ref
	Actions []string
}

// GrantToActor returns the command to grant the the given actions to the given actor.
func GrantToActor(actorID uuid.UUID, ref aggregate.Ref, actions ...string) command.Cmd[grantRevokePayload] {
	return command.New(GrantToActorCmd, grantRevokePayload{Ref: ref, Actions: actions}, command.Aggregate(ActorAggregate, actorID))
}

// RevokeFromActor returns the command to revoke the given actions from the given actor.
func RevokeFromActor(actorID uuid.UUID, ref aggregate.Ref, actions ...string) command.Cmd[grantRevokePayload] {
	return command.New(RevokeFromActorCmd, grantRevokePayload{Ref: ref, Actions: actions}, command.Aggregate(ActorAggregate, actorID))
}

// GrantToRole returns the command to grant the the given actions to the given role.
func GrantToRole(roleID uuid.UUID, ref aggregate.Ref, actions ...string) command.Cmd[grantRevokePayload] {
	return command.New(GrantToRoleCmd, grantRevokePayload{Ref: ref, Actions: actions}, command.Aggregate(RoleAggregate, roleID))
}

// RevokeFromRole returns the command to revoke the the given actions from the given role.
func RevokeFromRole(roleID uuid.UUID, ref aggregate.Ref, actions ...string) command.Cmd[grantRevokePayload] {
	return command.New(RevokeFromRoleCmd, grantRevokePayload{Ref: ref, Actions: actions}, command.Aggregate(RoleAggregate, roleID))
}

// RegisterCommands registers the commands of the auth package into a registry.
func RegisterCommands(r *codec.Registry) {
	gr := codec.Gob(r)
	codec.GobRegister[any](gr, IdentifyActorCmd)
	codec.GobRegister[string](gr, IdentifyRoleCmd)
	codec.GobRegister[[]uuid.UUID](gr, GiveRoleToCmd)
	codec.GobRegister[[]uuid.UUID](gr, RemoveRoleFromCmd)
	codec.GobRegister[grantRevokePayload](gr, GrantToActorCmd)
	codec.GobRegister[grantRevokePayload](gr, RevokeFromActorCmd)
	codec.GobRegister[grantRevokePayload](gr, GrantToRoleCmd)
	codec.GobRegister[grantRevokePayload](gr, RevokeFromRoleCmd)
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

	grantActorErrors := command.MustHandle(ctx, bus, GrantToActorCmd, func(ctx command.Ctx[grantRevokePayload]) error {
		load := ctx.Payload()

		actors, err := actorRepos.Repository(UUIDActor)
		if err != nil {
			return fmt.Errorf("get %q repository: %w", UUIDActor, err)
		}

		return actors.Use(ctx, ctx.AggregateID(), func(a *Actor) error {
			return a.Grant(load.Ref, load.Actions...)
		})
	})

	revokeActorErrors := command.MustHandle(ctx, bus, RevokeFromActorCmd, func(ctx command.Ctx[grantRevokePayload]) error {
		load := ctx.Payload()

		actors, err := actorRepos.Repository(UUIDActor)
		if err != nil {
			return fmt.Errorf("get %q repository: %w", UUIDActor, err)
		}

		return actors.Use(ctx, ctx.AggregateID(), func(a *Actor) error {
			return a.Revoke(load.Ref, load.Actions...)
		})
	})

	grantRoleErrors := command.MustHandle(ctx, bus, GrantToRoleCmd, func(ctx command.Ctx[grantRevokePayload]) error {
		load := ctx.Payload()
		return roles.Use(ctx, ctx.AggregateID(), func(r *Role) error {
			return r.Grant(load.Ref, load.Actions...)
		})
	})

	revokeRoleErrors := command.MustHandle(ctx, bus, RevokeFromRoleCmd, func(ctx command.Ctx[grantRevokePayload]) error {
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
