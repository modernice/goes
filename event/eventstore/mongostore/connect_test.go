// +build mongostore

package mongostore_test

import (
	"context"
	"os"
	"testing"

	"github.com/modernice/goes/event/eventstore/mongostore"
	"github.com/modernice/goes/event/eventstore/mongostore/mongotest"
	"github.com/modernice/goes/event/test"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestStore_Connect_options(t *testing.T) {
	// given a store that's not connected yet
	enc := test.NewEncoder()
	store := mongostore.New(enc)

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
	store := mongotest.NewStore(enc, mongostore.URL(os.Getenv("MONGOSTORE_URL")))

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
