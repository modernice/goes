package memory

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/modernice/goes/persistence/model"
)

var _ model.Repository[model.Model[uuid.UUID], uuid.UUID] = (*ModelRepository[model.Model[uuid.UUID], uuid.UUID])(nil)

// ModelRepository is thread-safe in-memory model repository. Useful for testing.
type ModelRepository[Model model.Model[ID], ID model.ID] struct {
	modelRepositoryOptions
	mux    sync.RWMutex
	models map[ID]Model
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
	r.models[m.ModelID()] = m
	return nil
}

// Fetch returns the model from the repository if its exists. Otherwise
// model.ErrNotFound is returned.
func (r *ModelRepository[Model, ID]) Fetch(ctx context.Context, id ID) (Model, error) {
	r.mux.RLock()
	defer r.mux.RUnlock()
	if m, ok := r.models[id]; ok {
		return m, nil
	}

	if r.factory != nil {
		return r.factory(id).(Model), nil
	}

	var zero Model
	return zero, model.ErrNotFound
}

// Use fetches the given model from the repository, passes the model to the
// provided function and finally saves the model back to the repository.
func (r *ModelRepository[Model, ID]) Use(ctx context.Context, id ID, fn func(Model) error) error {
	m, err := r.Fetch(ctx, id)
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

// Delete deletes the given model from the repository.
func (r *ModelRepository[Model, ID]) Delete(ctx context.Context, m Model) error {
	r.mux.Lock()
	defer r.mux.Unlock()
	delete(r.models, m.ModelID())
	return nil
}
