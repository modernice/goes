//go:build nats

package nats_test

import (
	"os"
	"testing"

	"github.com/modernice/goes/backend/nats"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventbus/test"
)

func TestEventBus_JetStream(t *testing.T) {
	testEventBus(t, "JetStream", newJetStreamBus)
	test.EventBus(t, newJetStreamBus)
}

func newJetStreamBus(enc event.Encoder) event.Bus {
	return nats.NewEventBus(enc, nats.EatErrors(), nats.Use(nats.JetStream()), nats.URL(os.Getenv("JETSTREAM_URL")))
}
