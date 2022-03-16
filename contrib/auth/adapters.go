package auth

import (
	"context"

	"github.com/google/uuid"
)

// RepositoryPermissionFetcher returns a PermissionFetcher that fetches
// permissions using the provided PermissionRepository.
func RepositoryPermissionFetcher(repo PermissionRepository) PermissionFetcherFunc {
	return func(ctx context.Context, actorID uuid.UUID) (PermissionsDTO, error) {
		perms, err := repo.Fetch(ctx, actorID)
		if err != nil {
			return PermissionsDTO{}, err
		}
		return perms.PermissionsDTO, nil
	}
}

// PermissionFetcherFunc allows a function to be used as a PermissionFetcher.
type PermissionFetcherFunc func(context.Context, uuid.UUID) (PermissionsDTO, error)

func (fetch PermissionFetcherFunc) Fetch(ctx context.Context, actorID uuid.UUID) (PermissionsDTO, error) {
	return fetch(ctx, actorID)
}

// ClientPermissionFetcher returns a PermissionFetcher that fetches permissions
// using the provided client. ClientPermissionFetcher simply returns the
// client.Permissions method, which is a PermissionFetcherFunc.
func ClientPermissionFetcher(client QueryClient) PermissionFetcherFunc {
	return client.Permissions
}

// ClientLookup returns a Lookup that uses the provided client to do the lookups.
func ClientLookup(client QueryClient) Lookup {
	return clientLookup{client}
}

type clientLookup struct{ client QueryClient }

func (l clientLookup) Actor(ctx context.Context, sid string) (uuid.UUID, bool) {
	id, err := l.client.LookupActor(ctx, sid)
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}

func (l clientLookup) Role(ctx context.Context, name string) (uuid.UUID, bool) {
	id, err := l.client.LookupRole(ctx, name)
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}
