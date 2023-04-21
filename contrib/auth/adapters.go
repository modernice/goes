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

// Fetch retrieves permissions for a given actor using the PermissionRepository
// provided to the RepositoryPermissionFetcher. It returns a PermissionsDTO and
// an error if any.
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

// Actor is a function that implements the Actor method of the Lookup interface.
// It uses the provided client to lookup actors using their sid identifier. It
// returns the actor's uuid.UUID and a boolean indicating if the lookup was
// successful or not.
func (l clientLookup) Actor(ctx context.Context, sid string) (uuid.UUID, bool) {
	id, err := l.client.LookupActor(ctx, sid)
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}

// Role returns the unique identifier for the role with the specified name,
// fetched using the provided QueryClient. It is a method of clientLookup type
// which implements Lookup interface.
func (l clientLookup) Role(ctx context.Context, name string) (uuid.UUID, bool) {
	id, err := l.client.LookupRole(ctx, name)
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}
