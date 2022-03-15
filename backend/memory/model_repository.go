package memory

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/persistence/model"
)

var _ model.Repository[model.Model[uuid.UUID], uuid.UUID] = (*ModelRepository[model.Model[uuid.UUID], uuid.UUID])(nil)

// ModelRepository is thread-safe in-memory model repository. Useful for testing.
type ModelRepository[Model model.Model[ID], ID model.ID] struct {
	modelRepositoryOptions
	mux    sync.RWMutex
	models map[ID]Model

	busyMux sync.RWMutex
	busy    map[ID]struct{}
}

// ModelRepositoryOption is an option for a model repository.
type ModelRepositoryOption func(*modelRepositoryOptions)

type modelRepositoryOptions struct {
	factory func(any) any
}

// ModelFactory returns a ModelRepositoryOption that provides a factory function
// for the models to a model repository. The repository will use the function to
// create the model if it doesn't exist in the repository instead of returning
// model.ErrNotFound.
func ModelFactory[Model model.Model[ID], ID model.ID](factory func(ID) Model) ModelRepositoryOption {
	return func(o *modelRepositoryOptions) {
		o.factory = func(id any) any {
			return factory(id.(ID))
		}
	}
}

// NewModelReository returns an in-memory model repository.
func NewModelRepository[Model model.Model[ID], ID model.ID](opts ...ModelRepositoryOption) *ModelRepository[Model, ID] {
	var options modelRepositoryOptions
	for _, opt := range opts {
		opt(&options)
	}
	return &ModelRepository[Model, ID]{
		modelRepositoryOptions: options,
		models:                 make(map[ID]Model),
		busy:                   map[ID]struct{}{},
	}
}

// Models returns all models in the repository.
func (r *ModelRepository[Model, ID]) Models() map[ID]Model {
	r.mux.Lock()
	defer r.mux.Unlock()
	out := make(map[ID]Model)
	for id, m := range r.models {
		out[id] = m
	}
	return out
}

// Save saves the model to the repository.
func (r *ModelRepository[Model, ID]) Save(ctx context.Context, m Model) error {
	r.mux.Lock()
	defer r.mux.Unlock()
	r.save(m)
	return nil
}

func (r *ModelRepository[Model, ID]) save(m Model) {
	r.models[m.ModelID()] = m
}

// Fetch returns the model from the repository if its exists. Otherwise
// model.ErrNotFound is returned. If the model is currently in use by r.Use(),
// Fetch waits for the model to become free. If ctx is canceled before the model
// becomes free, ctx.Err() is returned.
func (r *ModelRepository[Model, ID]) Fetch(ctx context.Context, id ID) (Model, error) {
	unlock, err := r.acquireLock(ctx, id)
	if err != nil {
		var zero Model
		return zero, fmt.Errorf("acquire lock: %w", err)
	}
	defer unlock()

	return r.fetchWithoutAcquire(ctx, id)
}

func (r *ModelRepository[Model, ID]) fetchWithoutAcquire(ctx context.Context, id ID) (Model, error) {
	r.mux.RLock()
	m, ok := r.models[id]
	r.mux.RUnlock()
	if ok {
		return m, nil
	}

	if r.factory != nil {
		r.mux.Lock()
		defer r.mux.Unlock()

		// Check again if the model was created while we were waiting for the lock.
		if m, ok = r.models[id]; ok {
			return m, nil
		}

		out := r.factory(id).(Model)
		r.save(out)

		return out, nil
	}

	var zero Model
	return zero, model.ErrNotFound
}

// Use fetches the given model from the repository, calls the provided function
// with it, and finally saves the model back to the repository. Use ensures that
// a single model is not operated on by multiple goroutines at the same time to
// avoid optimistic concurrency issues.
func (r *ModelRepository[Model, ID]) Use(ctx context.Context, id ID, fn func(Model) error) error {
	unlock, err := r.acquireLock(ctx, id)
	if err != nil {
		return fmt.Errorf("acquire lock: %w", err)
	}
	defer unlock()

	m, err := r.fetchWithoutAcquire(ctx, id)
	if err != nil {
		return fmt.Errorf("fetch model: %w", err)
	}

	if err := fn(m); err != nil {
		return err
	}

	if err := r.Save(ctx, m); err != nil {
		return fmt.Errorf("save model: %w", err)
	}

	return nil
}

// acquireLock locks the model with the given id and returns a function that
// unlocks the model. If the model is already locked, it waits until the model
// is unlocked. If ctx is canceled before the lock was acquired, ctx.Err() is
// returned. acquireLock is used to avoid logical concurrency issues that would
// be caused by multiple goroutines trying to operate on the same model concurrently.
func (r *ModelRepository[Model, ID]) acquireLock(ctx context.Context, id ID) (func(), error) {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}

		r.busyMux.RLock()
		_, busy := r.busy[id]
		r.busyMux.RUnlock()

		if busy {
			continue
		}

		// Check again if the model was locked while we were waiting for the lock.
		r.busyMux.Lock()
		_, busy = r.busy[id]
		if busy {
			r.busyMux.Unlock()
			continue
		}

		r.busy[id] = struct{}{}
		r.busyMux.Unlock()

		return func() {
			r.busyMux.Lock()
			defer r.busyMux.Unlock()
			delete(r.busy, id)
		}, nil
	}
}

// Delete deletes the given model from the repository.
func (r *ModelRepository[Model, ID]) Delete(ctx context.Context, m Model) error {
	unlock, err := r.acquireLock(ctx, m.ModelID())
	if err != nil {
		return fmt.Errorf("acquire lock: %w", err)
	}
	defer unlock()

	r.mux.Lock()
	defer r.mux.Unlock()
	delete(r.models, m.ModelID())

	return nil
}
