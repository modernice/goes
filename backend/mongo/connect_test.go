//go:build mongostore

package mongo_test

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/modernice/goes/backend/mongo"
	"github.com/modernice/goes/backend/mongo/mongotest"
	"github.com/modernice/goes/event/test"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestStore_Connect_options(t *testing.T) {
	// given a store that's not connected yet
	enc := test.NewEncoder()
	store := mongo.NewEventStore[uuid.UUID](enc)

	// when connecting with an invalid mongo uri
	client, err := store.Connect(context.Background(), options.Client().ApplyURI("foo://bar:123"))

	// it should fail
	if err == nil {
		t.Fatalf("expected store.Connect to fail; got %#v", err)
	}

	if client != nil {
		t.Errorf("expected store.Connect to return a nil client; got %#v", client)
	}
}

func TestStore_Connect_client(t *testing.T) {
	enc := test.NewEncoder()
	store := mongotest.NewEventStore(enc, mongo.URL(os.Getenv("MONGOSTORE_URL")))

	client, err := store.Connect(context.Background())
	if err != nil {
		t.Fatalf("expected store.Connect to succeed; got %#v", err)
	}

	if client == nil {
		t.Fatalf("expected store.Connect to return a client; got %#v", err)
	}

	if err = client.Ping(context.Background(), nil); err != nil {
		t.Errorf("expected client.Ping to succeed; got %#v", err)
	}
}
