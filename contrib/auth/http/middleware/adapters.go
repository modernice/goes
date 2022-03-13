package middleware

import (
	"context"

	"github.com/google/uuid"
	"github.com/modernice/goes/contrib/auth"
)

// RepositoryPermissionFetcher returns a PermissionFetcher that fetches
// permissions using the provided PermissionRepository.
func RepositoryPermissionFetcher(repo auth.PermissionRepository) PermissionFetcherFunc {
	return func(ctx context.Context, actorID uuid.UUID) (auth.PermissionsDTO, error) {
		perms, err := repo.Fetch(ctx, actorID)
		if err != nil {
			return auth.PermissionsDTO{}, err
		}
		return perms.PermissionsDTO, nil
	}
}

// ClientPermissionFetcher returns a PermissionFetcher that fetches permissions
// using the provided client.
func ClientPermissionFetcher(client auth.Client) PermissionFetcherFunc {
	return client.Permissions
}

// PermissionFetcherFunc allows a function to be used as a PermissionFetcher.
type PermissionFetcherFunc func(context.Context, uuid.UUID) (auth.PermissionsDTO, error)

func (fetch PermissionFetcherFunc) Fetch(ctx context.Context, actorID uuid.UUID) (auth.PermissionsDTO, error) {
	return fetch(ctx, actorID)
}

// ClientLookup returns a Lookup that uses the provided client to do the lookups.
func ClientLookup(client auth.Client) Lookup {
	return clientLookup{client}
}

type clientLookup struct{ client auth.Client }

func (l clientLookup) Actor(ctx context.Context, sid string) (uuid.UUID, bool) {
	id, err := l.client.LookupActor(ctx, sid)
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}
