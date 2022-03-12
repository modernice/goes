package middleware

import (
	"context"

	"github.com/google/uuid"
	"github.com/modernice/goes/contrib/auth"
)

// PermissionRepositoryFetcher returns a PermissionFetcher that fetches
// permissions using the provided PermissionRepository.
func PermissionRepositoryFetcher(repo auth.PermissionRepository) PermissionFetcher {
	return permissionRepoFetcher{repo}
}

type permissionRepoFetcher struct{ repo auth.PermissionRepository }

func (repo permissionRepoFetcher) Fetch(ctx context.Context, actorID uuid.UUID) (auth.PermissionsDTO, error) {
	perms, err := repo.repo.Fetch(ctx, actorID)
	if err != nil {
		return auth.PermissionsDTO{}, err
	}
	return perms.PermissionsDTO, nil
}
