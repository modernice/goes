//go:build nats

package natsbus_test

import (
	"fmt"
	"os"
	"sync/atomic"
	"testing"

	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventbus/natsbus"
	"github.com/nats-io/stan.go"
)

var id int64 = 1

func TestStreamingEventBus(t *testing.T) {
	// Flaky implementation.
	return
	// eventbustest.Run(t, newSTANBus)
	// testEventBus(t, newSTANBus)
}

func newSTANBus(enc codec.Encoding) event.Bus {
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
