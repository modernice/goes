//go:build mongosnap

package mongosnap_test

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate/snapshot"
	"github.com/modernice/goes/aggregate/snapshot/mongosnap"
	"github.com/modernice/goes/aggregate/snapshot/storetest"
)

var (
	storeID int64
)

func TestStore(t *testing.T) {
	t.Run("mongodb", func(t *testing.T) {
		storetest.Run(t, newStore, uuid.New)
	})
}

func TestStore_Connect(t *testing.T) {
	url := os.Getenv("MONGOSNAP_URL")
	s := mongosnap.New[uuid.UUID](mongosnap.URL(url))
	client, err := s.Connect(context.Background())
	if err != nil {
		t.Errorf("Connect shouldn't fail; failed with %q", err)
	}
	if err := client.Ping(context.Background(), nil); err != nil {
		t.Errorf("Ping failed with %q", err)
	}
}

func newStore() snapshot.Store[uuid.UUID] {
	id := atomic.AddInt64(&storeID, 1)
	return mongosnap.New[uuid.UUID](
		mongosnap.Database(fmt.Sprintf("snapshot_%d", id)),
		mongosnap.URL(os.Getenv("MONGOSNAP_URL")),
	)
}
