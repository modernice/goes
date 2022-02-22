//go:build mongo

package mongo_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/modernice/goes/backend/mongo"
	"github.com/modernice/goes/persistence/model"
	"go.mongodb.org/mongo-driver/bson/primitive"
	gomongo "go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestModelRepository_Save_Fetch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	col := connect(t)

	m := &basicModel{
		ID:  primitive.NewObjectID(),
		Foo: "foo",
	}

	r := mongo.NewModelRepository[*basicModel, primitive.ObjectID](col)

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
	col := connect(t)
	testModelRepository_Use(t, mongo.NewModelRepository[*basicModel, primitive.ObjectID](col))
}

func TestModelRepository_Use_Transaction(t *testing.T) {
	col := connect(t)
	testModelRepository_Use(t, mongo.NewModelRepository[*basicModel, primitive.ObjectID](col, mongo.ModelTransactions(true)))
}

func testModelRepository_Use(t *testing.T, r *mongo.ModelRepository[*basicModel, primitive.ObjectID]) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	m := &basicModel{
		ID:  primitive.NewObjectID(),
		Foo: "foo",
	}

	if err := r.Save(ctx, m); err != nil {
		t.Fatalf("failed to save model: %v", err)
	}

	if err := r.Use(ctx, m.ModelID(), func(m *basicModel) error {
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

	col := connect(t)

	r := mongo.NewModelRepository[*basicModel, primitive.ObjectID](col)

	m := &basicModel{
		ID:  primitive.NewObjectID(),
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

func TestModelRepository_CustomID(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	col := connect(t)
	r := mongo.NewModelRepository[*uuidModel, uuid.UUID](col, mongo.ModelIDKey("customid"))

	m := &uuidModel{
		ID:  uuid.New(),
		Foo: "foo",
	}

	if err := r.Save(ctx, m); err != nil {
		t.Fatalf("failed to save model: %v", err)
	}

	fetched, err := r.Fetch(ctx, m.ModelID())
	if err != nil {
		t.Fatalf("Fetch() failed with %q", err)
	}

	if fetched.ModelID() != m.ModelID() {
		t.Fatalf("fetched model has wrong id. %v != %v", fetched.ModelID(), m.ModelID())
	}

	if fetched.Foo != m.Foo {
		t.Fatalf("fetched model has wrong data. %v != %v", fetched.Foo, m.Foo)
	}
}

func TestModelRepository_CustomID_InvalidKey(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	col := connect(t)
	r := mongo.NewModelRepository[*uuidModel, uuid.UUID](col)

	m := &uuidModel{
		ID:  uuid.New(),
		Foo: "foobar",
	}

	if err := r.Save(ctx, m); err != nil {
		t.Fatalf("failed to save model: %v", err)
	}

	fetched, err := r.Fetch(ctx, m.ModelID())
	if err != nil {
		t.Fatalf("Fetch() failed with %q", err)
	}

	if fetched.ModelID() != m.ModelID() {
		t.Fatalf("fetched model has wrong id. %v != %v", fetched.ModelID(), m.ModelID())
	}

	if fetched.Foo != "foobar" {
		t.Fatalf("fetched model has wrong data. %v != %v", fetched.Foo, "foobar")
	}
}

func TestModelRepository_Fetch_ModelFactory_ErrNotFound(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	col := connect(t)
	r := mongo.NewModelRepository[*uuidModel, uuid.UUID](col, mongo.ModelFactory(func(id uuid.UUID) *uuidModel {
		return &uuidModel{
			ID:  id,
			Foo: "baz",
		}
	}, false))

	id := uuid.New()

	if _, err := r.Fetch(ctx, id); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("Fetch() should fail with %q; got %q", model.ErrNotFound, err)
	}
}

func TestModelRepository_Fetch_ModelFactory_CreateIfNotFound(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	col := connect(t)
	r := mongo.NewModelRepository[*uuidModel, uuid.UUID](col, mongo.ModelFactory(func(id uuid.UUID) *uuidModel {
		return &uuidModel{
			ID:  id,
			Foo: "baz",
		}
	}, true))

	id := uuid.New()

	m, err := r.Fetch(ctx, id)
	if err != nil {
		t.Fatalf("Fetch() failed with %q", err)
	}

	if m.ModelID() != id {
		t.Fatalf("ModelID() should return %q; got %q", id, m.ModelID())
	}

	if m.Foo != "baz" {
		t.Fatalf("Foo should be %q; is %q", "baz", m.Foo)
	}
}

func connect(t *testing.T) *gomongo.Collection {
	client, err := gomongo.Connect(context.Background(), options.Client().ApplyURI(os.Getenv("MONGOMODEL_URL")))
	if err != nil {
		t.Fatalf("connect to mongo: %v", err)
		return nil
	}

	db := client.Database("modeltest")
	col := db.Collection("models")

	return col
}

type basicModel struct {
	ID  primitive.ObjectID `bson:"_id"`
	Foo string
}

func (m basicModel) ModelID() primitive.ObjectID {
	return m.ID
}

type uuidModel struct {
	ID  uuid.UUID `bson:"customid"`
	Foo string
}

func (m uuidModel) ModelID() uuid.UUID {
	return m.ID
}
