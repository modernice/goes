//go:build mongostore

package mongostore_test

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventstore/mongostore"
	"github.com/modernice/goes/event/eventstore/mongostore/mongotest"
	"github.com/modernice/goes/event/test"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestClient(t *testing.T) {
	client, err := mongo.Connect(
		context.Background(),
		options.Client().ApplyURI(os.Getenv("MONGOSTORE_URL")),
	)
	if err != nil {
		t.Fatalf("mongo.Connect: %v", err)
	}

	store := mongostore.New(test.NewEncoder(), mongostore.Client(client))
	if _, err = store.Connect(context.Background()); err != nil {
		t.Fatalf("expected store.Connect to succeed; got %#v", err)
	}

	if cclient := store.Client(); cclient != client {
		t.Errorf("store.Client returned the wrong client\n\nwant: %#v\n\ngot: %#v", client, cclient)
	}

	if db := store.Database(); db == nil {
		t.Errorf("expected store.Database not to return %#v; got %#v", (*mongo.Database)(nil), db)
	}

	if col := store.Collection(); col == nil {
		t.Errorf("expected store.Collection not to return %#v; got %#v", (*mongo.Collection)(nil), col)
	}
}

func TestDatabase(t *testing.T) {
	store := mongotest.NewStore(
		test.NewEncoder(),
		mongostore.Database("event_customdb"),
		mongostore.URL(os.Getenv("MONGOSTORE_URL")),
	)

	if _, err := store.Connect(context.Background()); err != nil {
		t.Fatalf("expected store.Connect to succeed; got %#v", err)
	}

	db := store.Database()
	if name := db.Name(); name != "event_customdb" {
		t.Fatalf("expected store.Database().Name() to return %q; got %q", "event_customdb", name)
	}
}

func TestCollection(t *testing.T) {
	store := mongotest.NewStore(
		test.NewEncoder(),
		mongostore.Collection("custom"),
		mongostore.URL(os.Getenv("MONGOSTORE_URL")),
	)
	evt := event.New("foo", test.FooEventData{A: "foo"}, event.Aggregate(uuid.New(), "foo", 1))
	if err := store.Insert(context.Background(), evt); err != nil {
		t.Fatalf("store.Insert: %#v", err)
	}

	found, err := store.Find(context.Background(), evt.ID())
	if err != nil {
		t.Fatalf("expected store.Find to succeed; got %#v", err)
	}
	if !event.Equal(evt, found) {
		t.Errorf("store.Find returned the wrong event\n\nwant: %#v\n\ngot: %#v", evt, found)
	}

	if col := store.Collection(); col.Name() != "custom" {
		t.Errorf("expected store.Collection().Name() to return %q; got %q", "custom", col.Name())
	}
}
