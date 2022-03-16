package auth

import (
	"context"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
)

// Client defines the service client for the authorization module.
//
// Client is implemented by goes/contrib/auth/authrpc.Client.
type Client interface {
	// Permissions returns the permission read-model of the given actor.
	Permissions(ctx context.Context, actorID uuid.UUID) (PermissionsDTO, error)

	// Allows returns whether the given actor has the permission to perform the
	// given action on the given aggregate.
	Allows(ctx context.Context, actorID uuid.UUID, ref aggregate.Ref, action string) (bool, error)

	// LookupActor looks up the aggregate id of the actor with the given
	// formatted actor id.
	LookupActor(ctx context.Context, sid string) (uuid.UUID, error)
}

// PermissionFetcher fetches permissions of actors.
type PermissionFetcher interface {
	Fetch(context.Context, uuid.UUID) (PermissionsDTO, error)
}

// Lookup provides lookups of actor ids and role ids.
type Lookup interface {
	Actor(context.Context, string) (uuid.UUID, bool)
}
