package mongosnap

import (
	"context"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func TestDatabase(t *testing.T) {
	s := newStore(Database("foo"))
	if _, err := s.Connect(context.Background()); err != nil {
		t.Fatalf("Connect shouldn't fail; failed with %q", err)
	}

	if s.db.Name() != "foo" {
		t.Errorf("database should have name %q; got %q", "foo", s.db.Name())
	}
}

func TestCollection(t *testing.T) {
	s := newStore(Collection("foo"))
	if _, err := s.Connect(context.Background()); err != nil {
		t.Fatalf("Connect shouldn't fail; failed with %q", err)
	}

	if s.col.Name() != "foo" {
		t.Errorf("collection should have name %q; got %q", "foo", s.db.Name())
	}
}

func newStore(opts ...Option) *Store {
	client, err := mongo.Connect(context.Background())
	if err != nil {
		panic(err)
	}
	names, err := client.ListDatabaseNames(context.Background(), bson.M{"name": "snapshot"})
	if err != nil {
		panic(err)
	}
	for _, name := range names {
		db := client.Database(name)
		names, err := db.ListCollectionNames(context.Background(), bson.M{})
		if err != nil {
			panic(err)
		}
		for _, name := range names {
			if err = db.Collection(name).Drop(context.Background()); err != nil {
				panic(err)
			}
		}
	}
	return New(opts...)
}
