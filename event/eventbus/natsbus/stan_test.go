//go:build nats
// +build nats

package natsbus_test

import (
	"fmt"
	"os"
	"sync/atomic"
	"testing"

	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventbus/natsbus"
	"github.com/nats-io/stan.go"
)

var id int64 = 1

func TestStreamingEventBus(t *testing.T) {
	// testEventBus(t, "Streaming", newSTANBus)
}

func newSTANBus(enc event.Encoder) event.Bus {
	n := atomic.AddInt64(&id, 1)
	return natsbus.New(
		enc,
		natsbus.Use(natsbus.Streaming(
			"test-cluster",
			fmt.Sprintf("stan-test-%d", n),
			stan.NatsURL(os.Getenv("STAN_URL")),
		)),
		natsbus.EatErrors(),
	)
}
