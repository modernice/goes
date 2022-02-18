package memory

// // Repository is an in-memory repository for any kind of model.
// type ModelRepository[M model.Model[ID], ID goes.ID] struct {
// 	mux    sync.RWMutex
// 	models map[ID][]byte
// }

// // NewRepository returns a new in-memory model repository.
// func NewModelRepository[M model.Model[ID], ID goes.ID]() *ModelRepository[M, ID] {
// 	return &ModelRepository[M, ID]{models: make(map[ID][]byte)}
// }

// // Save saves the given model.
// func (r *ModelRepository[Model, ID]) Save(ctx context.Context, m Model) error {
// 	var buf bytes.Buffer
// 	if err := gob.NewEncoder(&buf).Encode(m); err != nil {
// 		return fmt.Errorf("encode model: %w", err)
// 	}

// 	r.mux.Lock()
// 	defer r.mux.Unlock()
// 	r.models[m.ID()] = buf.Bytes()

// 	return nil
// }

// func (r *ModelRepository[Model, ID]) Fetch(ctx context.Context, id ID) (Model, error) {
// 	r.mux.RLock()
// 	defer r.mux.RUnlock()

// 	b, ok := r.models[id]
// 	if !ok {
// 		var zero Model
// 		return zero, model.ErrNotFound
// 	}

// 	var m Model
// 	if err := gob.NewDecoder(bytes.NewReader(b)).Decode(&m); err != nil {
// 		return m, fmt.Errorf("decode model: %w", err)
// 	}

// 	return m, nil
// }

// func (r *ModelRepository[Model, ID]) Use(ctx context.Context, id ID, fn func(Model) error) error {
// 	m, err := r.Fetch(ctx, id)
// 	if err != nil {
// 		return fmt.Errorf("fetch model: %w [id=%v]", err, id)
// 	}

// 	if err := fn(m); err != nil {
// 		return err
// 	}

// 	if err := r.Save(ctx, m); err != nil {
// 		return fmt.Errorf("save model: %w", err)
// 	}

// 	return nil
// }

// func (r *ModelRepository[Model, ID]) Delete(ctx context.Context, m Model) error {
// 	r.mux.Lock()
// 	defer r.mux.Unlock()
// 	delete(r.models, m.ID())
// 	return nil
// }
