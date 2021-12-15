//go:build nats

package nats_test

import (
	"os"
	"testing"

	"github.com/modernice/goes/backend/nats"
	"github.com/modernice/goes/backend/testing/eventbustest"
	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/event"
)

func TestEventBus_JetStream(t *testing.T) {
	t.Run("JetStream", func(t *testing.T) {
		eventbustest.Run(t, newJetStreamBus)
		testEventBus(t, newJetStreamBus)
	})
}

func newJetStreamBus(enc codec.Encoding) event.Bus {
	return nats.NewEventBus(enc, nats.EatErrors(), nats.Use(nats.JetStream()), nats.URL(os.Getenv("JETSTREAM_URL")))
}
