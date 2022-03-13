package auth

import (
	"context"

	"github.com/google/uuid"
)

// Client defines the service client for the authorization module.
//
// Client is implemented by goes/contrib/auth/authrpc.Client.
type Client interface {
	// Permissions returns the permission read-model of the given actor.
	Permissions(ctx context.Context, actorID uuid.UUID) (PermissionsDTO, error)

	// LookupActor looks up the aggregate id of the actor with the given
	// formatted actor id.
	LookupActor(ctx context.Context, sid string) (uuid.UUID, error)
}
