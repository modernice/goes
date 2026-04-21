//go:build nats

package natsjs

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/modernice/goes/backend/testing/eventbustest"
	"github.com/modernice/goes/backend/testing/eventstoretest"
	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/event"
	"github.com/nats-io/nats.go"
)

var (
	sharedConn   *nats.Conn
	sharedConnMu sync.Mutex

	storesMu sync.Mutex
	stores   []*EventStore
)

func getSharedConn() *nats.Conn {
	sharedConnMu.Lock()
	defer sharedConnMu.Unlock()
	if sharedConn == nil {
		var err error
		sharedConn, err = nats.Connect(natsURL())
		if err != nil {
			panic(fmt.Sprintf("connect to nats: %v", err))
		}
	}
	return sharedConn
}

func TestMain(m *testing.M) {
	code := m.Run()

	ctx := context.Background()
	storesMu.Lock()
	for _, s := range stores {
		s.cleanupForTests(ctx)
	}
	storesMu.Unlock()

	sharedConnMu.Lock()
	if sharedConn != nil {
		sharedConn.Close()
	}
	sharedConnMu.Unlock()

	os.Exit(code)
}

func TestEventStore(t *testing.T) {
	eventstoretest.Run(t, "NATSJetStream", newStore)
}

func TestEventBus(t *testing.T) {
	eventbustest.RunCore(t, newBus)
	eventbustest.RunWildcard(t, newBus)
}

func newStore(enc codec.Encoding) event.Store {
	return makeTestStore(enc)
}

func newBus(enc codec.Encoding) event.Bus {
	return makeTestStore(enc)
}

func makeTestStore(enc codec.Encoding) *EventStore {
	ns := uniqueNS()
	s := NewEventStore(
		enc,
		Conn(getSharedConn()),
		Namespace(ns),
		KVBucket(ns+"_idx"),
	)

	storesMu.Lock()
	stores = append(stores, s)
	storesMu.Unlock()

	return s
}

func natsURL() string {
	if url := os.Getenv("JETSTREAM_URL"); url != "" {
		return url
	}
	if url := os.Getenv("NATS_URL"); url != "" {
		return url
	}
	return "nats://localhost:4222"
}

func uniqueNS() string {
	b := make([]byte, 4)
	rand.Read(b)
	return fmt.Sprintf("t%x", b)
}
