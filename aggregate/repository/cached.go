package repository

import (
	"context"
	"sync"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"golang.org/x/exp/maps"
)

var _ aggregate.TypedRepository[aggregate.TypedAggregate] = (*CachedRepository[aggregate.TypedAggregate])(nil)

// CachedRepository is a type that provides a caching layer over an underlying
// repository of typed aggregates. It stores fetched aggregates in memory to
// reduce the need for repeated fetches from the wrapped repository. It uses
// UUIDs as keys to access stored aggregates. CachedRepository is safe for
// concurrent use.
//
// CachedRepository currently only caches calls to Fetch.
type CachedRepository[Aggregate aggregate.TypedAggregate] struct {
	aggregate.TypedRepository[Aggregate]

	mux   sync.RWMutex
	cache map[uuid.UUID]Aggregate
}

// Cached returns a new CachedRepository. If the provided repository is already
// a CachedRepository, it is returned as is. Otherwise, a new CachedRepository
// is created with the provided repository as its underlying repository. The
// returned CachedRepository uses an in-memory cache to avoid unnecessary
// fetches from the underlying repository.
func Cached[Aggregate aggregate.TypedAggregate](repo aggregate.TypedRepository[Aggregate]) *CachedRepository[Aggregate] {
	if cr, ok := repo.(*CachedRepository[Aggregate]); ok {
		return cr
	}
	return &CachedRepository[Aggregate]{
		TypedRepository: repo,
		cache:           make(map[uuid.UUID]Aggregate),
	}
}

// Clear empties the cache of the CachedRepository. All aggregates currently
// held in memory are removed, and subsequent fetches will retrieve aggregates
// from the underlying TypedRepository. This operation is safe for concurrent
// use.
func (repo *CachedRepository[Aggregate]) Clear() {
	repo.mux.Lock()
	defer repo.mux.Unlock()
	maps.Clear(repo.cache)
}

// Fetch retrieves an aggregate of type Aggregate from the CachedRepository. If
// the aggregate is present in the cache, it's returned directly. Otherwise,
// Fetch retrieves the aggregate from the underlying TypedRepository, stores it
// in the cache for future retrievals, and then returns it. An error is returned
// if there was a problem fetching the aggregate from the TypedRepository.
func (repo *CachedRepository[Aggregate]) Fetch(ctx context.Context, id uuid.UUID) (Aggregate, error) {
	if a, ok := repo.cached(id); ok {
		return a, nil
	}

	repo.mux.Lock()
	defer repo.mux.Unlock()

	if cached, ok := repo.cache[id]; ok {
		return cached, nil
	}

	a, err := repo.TypedRepository.Fetch(ctx, id)
	if err != nil {
		return a, err
	}

	repo.cache[id] = a

	return a, nil
}

func (repo *CachedRepository[Aggregate]) cached(id uuid.UUID) (Aggregate, bool) {
	repo.mux.RLock()
	defer repo.mux.RUnlock()
	a, ok := repo.cache[id]
	return a, ok
}
