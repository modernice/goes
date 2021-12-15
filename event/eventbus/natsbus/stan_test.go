//go:build nats

package natsbus_test

import (
	"fmt"
	"os"
	"sync/atomic"
	"testing"

	"github.com/modernice/goes/backend/testing/eventbustest"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventbus/natsbus"
	"github.com/nats-io/stan.go"
)

var id int64 = 1

func TestStreamingEventBus(t *testing.T) {
	t.Run("Streaming", func(t *testing.T) {
		eventbustest.Run(t, newSTANBus)
		testEventBus(t, newSTANBus)
	})
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
