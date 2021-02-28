package mongosnap_test

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/modernice/goes/aggregate/snapshot"
	"github.com/modernice/goes/aggregate/snapshot/mongosnap"
	"github.com/modernice/goes/aggregate/snapshot/storetest"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

var (
	storeID int64
	client  *mongo.Client
)

func TestStore(t *testing.T) {
	defer cleanup()
	storetest.Run(t, newStore)
}

func newStore() snapshot.Store {
	id := atomic.AddInt64(&storeID, 1)
	s := mongosnap.New(mongosnap.Database(fmt.Sprintf("snapshot_%d", id)))
	c, err := s.Connect(context.Background())
	if err != nil {
		panic(err)
	}
	if client == nil {
		client = c
	}
	return s
}

func TestStore_Connect(t *testing.T) {
	s := mongosnap.New()
	client, err := s.Connect(context.Background())
	if err != nil {
		t.Errorf("Connect shouldn't fail; failed with %q", err)
	}
	if err := client.Ping(context.Background(), nil); err != nil {
		t.Errorf("Ping failed with %q", err)
	}
}

func cleanup() {
	names, err := client.ListDatabaseNames(context.Background(), bson.D{
		{Key: "name", Value: bson.D{
			{Key: "$regex", Value: "^(snapshot_.)|foo$"},
		}},
	})
	if err != nil {
		panic(err)
	}
	for _, name := range names {
		if err := client.Database(name).Drop(context.Background()); err != nil {
			panic(err)
		}
	}
}
