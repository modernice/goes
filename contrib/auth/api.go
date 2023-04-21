package auth

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/command"
)

var (
	_ CommandClient = (*commandBusClient)(nil)
	_ CommandClient = (*repositoryCommandClient)(nil)
)

// QueryClient defines the query client for the authorization module.
//
// QueryClient is implemented by goes/contrib/auth/authrpc.Client.
type QueryClient interface {
	// Permissions returns the permission read-model of the given actor.
	Permissions(ctx context.Context, actorID uuid.UUID) (PermissionsDTO, error)

	// Allows returns whether the given actor has the permission to perform the
	// given action on the given aggregate.
	Allows(ctx context.Context, actorID uuid.UUID, ref aggregate.Ref, action string) (bool, error)

	// LookupActor looks up the aggregate id of the actor with the given
	// formatted actor id.
	LookupActor(ctx context.Context, sid string) (uuid.UUID, error)

	// LookupRole looks up the agggregate id of the role with the given name.
	LookupRole(ctx context.Context, name string) (uuid.UUID, error)
}

// CommandClient defines the command client for the authorization module.
// It exposes the commands to grant and revoke permissions as an interface. Each
// of these commands is also available as a "standalone" command:
//
//   - auth.CommandClient.GrantToActor() -> auth.GrantToActor()
//   - auth.CommandClient.GrantToRole() -> auth.GrantToRole()
//   - auth.CommandClient.RevokeFromActor() -> auth.RevokeFromActor()
//   - auth.CommandClient.RevokeFromRole() -> auth.RevokeFromRole()
//
// Use the CommandBusClient() constructor to create a CommandClient from an
// underlying command bus. Alternatively, use the RepositoryCommandClient() to
// create a CommandClient from actor and role repositories, or use
// authrpc.NewClient() to create a gRPC CommandClient.
type CommandClient interface {
	// GrantToActor grants the given actor the permission to perform the given actions.
	GrantToActor(context.Context, uuid.UUID, aggregate.Ref, ...string) error

	// GrantToRole grants the given role the permission to perform the given actions.
	GrantToRole(context.Context, uuid.UUID, aggregate.Ref, ...string) error

	// RevokeFromActor revokes from the given actor the permission to perform the given actions.
	RevokeFromActor(context.Context, uuid.UUID, aggregate.Ref, ...string) error

	// RevokeFromRole revokes from the given role the permission to perform the given actions.
	RevokeFromRole(context.Context, uuid.UUID, aggregate.Ref, ...string) error
}

// PermissionFetcher fetches permissions of actors.
type PermissionFetcher interface {
	// Fetch fetches the permissions of the given actor.
	Fetch(context.Context, uuid.UUID) (PermissionsDTO, error)
}

// Lookup provides lookups of actor ids and role ids.
type Lookup interface {
	// Actor returns the aggregate id of the actor with the given string-formatted actor id.
	Actor(context.Context, string) (uuid.UUID, bool)

	// Role returns the aggregate id of the role with the given name.
	Role(context.Context, string) (uuid.UUID, bool)
}

// CommandBusClient returns a CommandClient that executes commands by
// dispatching them via the provided command bus. The provided dispatch options
// are applied to all dispatched commands.
func CommandBusClient(bus command.Bus, opts ...command.DispatchOption) CommandClient {
	return commandBusClient{
		bus:  bus,
		opts: opts,
	}
}

type commandBusClient struct {
	bus  command.Bus
	opts []command.DispatchOption
}

// GrantToActor grants the given actor the permission to perform the given
// actions on the specified aggregate. It is a method of the CommandClient
// interface that can be constructed using one of the available constructors.
func (client commandBusClient) GrantToActor(ctx context.Context, actorID uuid.UUID, ref aggregate.Ref, actions ...string) error {
	return client.bus.Dispatch(ctx, GrantToActor(actorID, ref, actions...).Any(), client.opts...)
}

// GrantToRole grants the given
// [role](https://pkg.go.dev/github.com/modernice/goes-contrib/auth#Role) the
// permission to perform the given actions on the specified
// [aggregate.Ref](https://pkg.go.dev/github.com/modernice/goes/aggregate#Ref).
func (client commandBusClient) GrantToRole(ctx context.Context, roleID uuid.UUID, ref aggregate.Ref, actions ...string) error {
	return client.bus.Dispatch(ctx, GrantToRole(roleID, ref, actions...).Any(), client.opts...)
}

// RevokeFromActor revokes from the given actor the permission to perform the
// given actions. It is a method of the CommandClient interface, which is
// implemented by commandBusClient and repositoryCommandClient.
func (client commandBusClient) RevokeFromActor(ctx context.Context, actorID uuid.UUID, ref aggregate.Ref, actions ...string) error {
	return client.bus.Dispatch(ctx, RevokeFromActor(actorID, ref, actions...).Any(), client.opts...)
}

// RevokeFromRole revokes from the given role the permission to perform the
// given actions. It takes a context, role ID, aggregate reference and one or
// more actions as arguments. It returns an error if the command execution
// fails.
func (client commandBusClient) RevokeFromRole(ctx context.Context, roleID uuid.UUID, ref aggregate.Ref, actions ...string) error {
	return client.bus.Dispatch(ctx, RevokeFromRole(roleID, ref, actions...).Any(), client.opts...)
}

// RepositoryCommandClient returns a CommandClient that executes commands directly
// on the Actor and Role aggregates within the provided repositories.
func RepositoryCommandClient(actors ActorRepositories, roles RoleRepository) CommandClient {
	return repositoryCommandClient{
		actors: actors,
		roles:  roles,
	}
}

type repositoryCommandClient struct {
	actors ActorRepositories
	roles  RoleRepository
}

// GrantToActor grants the given actor the permission to perform the given
// actions. It is a method of the CommandClient interface, which defines the
// command client for the authorization module. This interface exposes commands
// to grant and revoke permissions as an interface, each of which is also
// available as a "standalone" command. Use the RepositoryCommandClient
// constructor to create a CommandClient from actor and role repositories, or
// use authrpc.NewClient() to create a gRPC CommandClient.
func (client repositoryCommandClient) GrantToActor(ctx context.Context, actorID uuid.UUID, ref aggregate.Ref, actions ...string) error {
	if len(actions) == 0 {
		return nil
	}

	actors, err := client.actors.Repository(UUIDActor)
	if err != nil {
		return fmt.Errorf("get UUIDActor repository: %w", err)
	}

	return actors.Use(ctx, actorID, func(a *Actor) error {
		return a.Grant(ref, actions...)
	})
}

// GrantToRole grants the given role the permission to perform the given
// actions. It is a method of the CommandClient interface, which is implemented
// by repositoryCommandClient.
func (client repositoryCommandClient) GrantToRole(ctx context.Context, roleID uuid.UUID, ref aggregate.Ref, actions ...string) error {
	if len(actions) == 0 {
		return nil
	}

	return client.roles.Use(ctx, roleID, func(r *Role) error {
		return r.Grant(ref, actions...)
	})
}

// RevokeFromActor revokes from the given actor the permission to perform the
// given actions. This function is part of the CommandClient interface, which
// defines the command client for the authorization module. Use
// RepositoryCommandClient() to create a CommandClient from actor and role
// repositories.
func (client repositoryCommandClient) RevokeFromActor(ctx context.Context, actorID uuid.UUID, ref aggregate.Ref, actions ...string) error {
	if len(actions) == 0 {
		return nil
	}

	actors, err := client.actors.Repository(UUIDActor)
	if err != nil {
		return fmt.Errorf("get UUIDActor repository: %w", err)
	}

	return actors.Use(ctx, actorID, func(a *Actor) error {
		return a.Revoke(ref, actions...)
	})
}

// RevokeFromRole revokes from the given role the permission to perform the
// given actions. It is a CommandClient method that executes commands directly
// on the Role aggregates within the provided repositories.
func (client repositoryCommandClient) RevokeFromRole(ctx context.Context, roleID uuid.UUID, ref aggregate.Ref, actions ...string) error {
	if len(actions) == 0 {
		return nil
	}

	return client.roles.Use(ctx, roleID, func(r *Role) error {
		return r.Revoke(ref, actions...)
	})
}
