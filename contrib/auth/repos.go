package auth

import (
	"errors"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/repository"
	"github.com/modernice/goes/backend/memory"
	"github.com/modernice/goes/persistence/model"
)

var _ ActorRepositories = (*ActorRepositoryRegistry)(nil)

// ErrUnknownActorKind is returned by ParseKind() if the passed id type is not
// a builtin actor id type.
var ErrUnknownActorKind = errors.New("unknown actor kind")

// ActorRepository is the repository for Actors.
type ActorRepository = aggregate.TypedRepository[*Actor]

// ActorRepositories provides Actor repositories for different kinds of actors.
type ActorRepositories interface {
	// ParseKind parses actor kinds from ids.
	ParseKind(any) (string, error)

	// Repository returns the repository for the given actor kind.
	Repository(kind string) (ActorRepository, error)
}

// RoleRepository is the repository for Roles.
type RoleRepository = aggregate.TypedRepository[*Role]

// PermissionRepository is the repository for the permission read-models.
type PermissionRepository = model.Repository[*Permissions, uuid.UUID]

// NewUUIDActorRepository returns the repository for UUID-Actors.
func NewUUIDActorRepository(repo aggregate.Repository) ActorRepository {
	return repository.Typed(repo, NewUUIDActor)
}

// NewStringActorRepository returns the repository for string-Actors.
func NewStringActorRepository(repo aggregate.Repository) ActorRepository {
	return repository.Typed(repo, NewStringActor)
}

// NewRoleRepository returns the repository for Roles.
func NewRoleRepository(repo aggregate.Repository) RoleRepository {
	return repository.Typed(repo, NewRole)
}

// InMemoryPerissionRepository returns an in-memory repository for the
// permission read-models.
func InMemoryPermissionRepository() PermissionRepository {
	return memory.NewModelRepository[*Permissions, uuid.UUID](memory.ModelFactory(PermissionsOf))
}

// ActorRepositoryRegistry is a registry for Actor repositories of different kinds.
type ActorRepositoryRegistry struct {
	sync.RWMutex
	repos     map[string]ActorRepository
	parseKind func(any) (string, error)
}

// ParseKind is the builtin implementation of ActorRepositories.ParseKind and is
// used by default if the provided `parseKind` argument that is passed to
// NewActorRepositories is nil. ParseKind supports parsing of string-Actors and
// UUID-Actors. If v is neither a string nor a UUID, an error that satisfies
// errors.Is(err, ErrUnknownActorKind) is returned. To add support for
// custom actor kinds, pass a custom ParseKind implementation to
// NewActorRepositories.
func ParseKind(v any) (string, error) {
	switch v.(type) {
	case string:
		return StringActor, nil
	case uuid.UUID:
		return UUIDActor, nil
	default:
		return "", fmt.Errorf("%w: %s", ErrUnknownActorKind, v)
	}
}

// NewActorRepositories returns an ActorRepositoryRegistry that provides
// repositories for builtin actor kinds (StringActor and UUIDActor).
func NewActorRepositories(repo aggregate.Repository, parseKind func(any) (string, error)) *ActorRepositoryRegistry {
	out := NewEmptyActorRepositories(parseKind)
	out.Add(StringActor, NewStringActorRepository(repo))
	out.Add(UUIDActor, NewUUIDActorRepository(repo))
	return out
}

// NewEmptyActorRepositories returns a fresh ActorRepositoryRegistry.
// The provided parseKind function is used to implement
// ActorRepositories.ParseKind. If parseKind is nil, it is set to ParseKind.
func NewEmptyActorRepositories(parseKind func(any) (string, error)) *ActorRepositoryRegistry {
	return &ActorRepositoryRegistry{
		repos: make(map[string]ActorRepository),
		parseKind: func(v any) (string, error) {
			k, err := ParseKind(v)
			if err == nil {
				return k, nil
			}

			if parseKind == nil {
				return k, err
			}

			return parseKind(v)
		},
	}
}

// ParseKind implements ActorRepositories.ParseKind.
func (repos *ActorRepositoryRegistry) ParseKind(v any) (string, error) {
	return repos.parseKind(v)
}

// Add adds the ActorRepository for the given actor kind to the registry.
func (repos *ActorRepositoryRegistry) Add(kind string, repo ActorRepository) {
	repos.Lock()
	defer repos.Unlock()
	repos.repos[kind] = repo
}

// Repository returns the actor repository for the given actor kind.
func (repos *ActorRepositoryRegistry) Repository(kind string) (ActorRepository, error) {
	repos.RLock()
	defer repos.RUnlock()
	if repo, ok := repos.repos[kind]; ok {
		return repo, nil
	}
	return nil, fmt.Errorf("unregistered kind: %s", kind)
}
