package memory_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/modernice/goes/backend/memory"
	"github.com/modernice/goes/persistence/model"
)

func TestModelRepository_Save_Fetch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	m := &uuidModel{
		ID:  uuid.New(),
		Foo: "foo",
	}

	r := memory.NewModelRepository[*uuidModel, uuid.UUID]()

	if _, err := r.Fetch(ctx, m.ModelID()); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("Fetch() should fail with %q for a model that does not exist in the database; got %q", model.ErrNotFound, err)
	}

	if err := r.Save(ctx, m); err != nil {
		t.Fatalf("failed to save model: %v", err)
	}

	fetched, err := r.Fetch(ctx, m.ModelID())
	if err != nil {
		t.Fatalf("failed to fetch model: %v", err)
	}

	if fetched.ModelID() != m.ModelID() {
		t.Fatalf("fetched model has wrong id: %v != %v", fetched.ModelID(), m.ModelID())
	}

	if fetched.Foo != m.Foo {
		t.Fatalf(`"Foo" field should be %q; is %q`, fetched.Foo, m.Foo)
	}
}

func TestModelRepository_Use(t *testing.T) {
	r := memory.NewModelRepository[*uuidModel, uuid.UUID]()
	testModelRepository_Use(t, r)
}

func TestModelRepository_Use_Transaction(t *testing.T) {
	r := memory.NewModelRepository[*uuidModel, uuid.UUID]()
	testModelRepository_Use(t, r)
}

func testModelRepository_Use(t *testing.T, r *memory.ModelRepository[*uuidModel, uuid.UUID]) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	m := &uuidModel{
		ID:  uuid.New(),
		Foo: "foo",
	}

	if err := r.Save(ctx, m); err != nil {
		t.Fatalf("failed to save model: %v", err)
	}

	if err := r.Use(ctx, m.ModelID(), func(m *uuidModel) error {
		m.Foo = "bar"
		return nil
	}); err != nil {
		t.Fatalf("Use() failed with %q", err)
	}

	fetched, err := r.Fetch(ctx, m.ModelID())
	if err != nil {
		t.Fatalf("failed to fetch model: %v", err)
	}

	if fetched.Foo != "bar" {
		t.Fatalf("Foo should be %q; is %q", "bar", fetched.Foo)
	}
}

func TestModelRepository_Delete(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r := memory.NewModelRepository[*uuidModel, uuid.UUID]()

	m := &uuidModel{
		ID:  uuid.New(),
		Foo: "foo",
	}

	if err := r.Save(ctx, m); err != nil {
		t.Fatalf("failed to save model: %v", err)
	}

	if err := r.Delete(ctx, m); err != nil {
		t.Fatalf("Delete() failed with %q", err)
	}

	if _, err := r.Fetch(ctx, m.ModelID()); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("fetching a deleted model should return %q; got %q", model.ErrNotFound, err)
	}
}

func TestModelRepository_Fetch_ModelFactory(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r := memory.NewModelRepository[*uuidModel, uuid.UUID](memory.ModelFactory(func(id uuid.UUID) *uuidModel {
		return &uuidModel{
			ID:  id,
			Foo: "baz",
		}
	}))

	id := uuid.New()

	m, err := r.Fetch(ctx, id)
	if errors.Is(err, model.ErrNotFound) {
		t.Fatalf("Fetch() failed with %q", err)
	}

	if m.Foo != "baz" {
		t.Fatalf("Foo should be %q; is %q", "baz", m.Foo)
	}
}

type uuidModel struct {
	ID  uuid.UUID `bson:"customid"`
	Foo string
}

// ModelID returns the unique identifier of the uuidModel.
func (m uuidModel) ModelID() uuid.UUID {
	return m.ID
}
