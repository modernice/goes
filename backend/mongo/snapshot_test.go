//go:build mongo

package mongo_test

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"testing"

	"github.com/modernice/goes/aggregate/snapshot"
	"github.com/modernice/goes/aggregate/snapshot/storetest"
	"github.com/modernice/goes/backend/mongo"
)

var (
	storeID int64
)

func TestStore(t *testing.T) {
	t.Run("mongodb", func(t *testing.T) {
		storetest.Run(t, newStore)
	})
}

func TestStore_Connect(t *testing.T) {
	url := os.Getenv("MONGOSNAP_URL")
	s := mongo.NewSnapshotStore(mongo.SnapshotURL(url))
	client, err := s.Connect(context.Background())
	if err != nil {
		t.Errorf("Connect shouldn't fail; failed with %q", err)
	}
	if err := client.Ping(context.Background(), nil); err != nil {
		t.Errorf("Ping failed with %q", err)
	}
}

func newStore() snapshot.Store {
	id := atomic.AddInt64(&storeID, 1)
	return mongo.NewSnapshotStore(
		mongo.SnapshotDatabase(fmt.Sprintf("snapshot_%d", id)),
		mongo.SnapshotURL(os.Getenv("MONGOSNAP_URL")),
	)
}
