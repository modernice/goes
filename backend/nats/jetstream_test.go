//go:build nats

package nats_test

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"testing"

	"github.com/modernice/goes/backend/nats"
	"github.com/modernice/goes/backend/testing/eventbustest"
	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/event"
)

func TestEventBus_JetStream(t *testing.T) {
	t.Run("Plain", jetStreamTest(newJetStreamBus))
	t.Run("Durable", jetStreamTest(newDurableJetStreamBus))
	t.Run("Queue", jetStreamTest(newQueueGroupJetStreamBus))
	t.Run("Durable+Queue", jetStreamTest(newDurableQueueGroupJetStreamBus))
}

func jetStreamTest(newBus func(codec.Encoding) event.Bus) func(t *testing.T) {
	return func(t *testing.T) {
		eventbustest.RunCore(t, newBus, eventbustest.Cleanup(cleanup))
		eventbustest.RunWildcard(t, newBus, eventbustest.Cleanup(cleanup))
		testEventBus(t, newBus)
	}
}

var n int64

func newJetStreamBus(enc codec.Encoding) event.Bus {
	return nats.NewEventBus(
		enc,
		nats.EatErrors(),
		nats.Use(nats.JetStream()),
		nats.URL(os.Getenv("JETSTREAM_URL")),
		nats.SubjectPrefix("jetstream:"),
	)
}

func newDurableJetStreamBus(enc codec.Encoding) event.Bus {
	return nats.NewEventBus(
		enc,
		nats.EatErrors(),
		nats.Use(nats.JetStream(nats.Durable("durable"))),
		nats.URL(os.Getenv("JETSTREAM_URL")),
		nats.SubjectPrefix("jetstream_durable:"),
	)
}

func newQueueGroupJetStreamBus(enc codec.Encoding) event.Bus {
	return nats.NewEventBus(
		enc,
		nats.EatErrors(),
		nats.Use(nats.JetStream()),
		nats.URL(os.Getenv("JETSTREAM_URL")),
		nats.LoadBalancer(randomQueue()),
		nats.SubjectPrefix("jetstream_queue:"),
	)
}

func newDurableQueueGroupJetStreamBus(enc codec.Encoding) event.Bus {
	return nats.NewEventBus(
		enc,
		nats.EatErrors(),
		nats.Use(nats.JetStream(nats.Durable("durable_queue"))),
		nats.URL(os.Getenv("JETSTREAM_URL")),
		nats.LoadBalancer(randomQueue()),
		nats.SubjectPrefix("jetstream_durable_queue:"),
	)
}

func randomQueue() string {
	buf := make([]byte, 8)
	rand.Read(buf)
	return fmt.Sprintf("%x", buf)
}

func cleanup(bus *nats.EventBus) error {
	js, err := bus.Connection().JetStream()
	if err != nil {
		return nil
	}

	for stream := range js.StreamNames() {
		for cons := range js.ConsumerNames(stream) {
			if err := js.DeleteConsumer(stream, cons); err != nil {
				return fmt.Errorf("delete %q consumer: %w", cons, err)
			}
		}

		if err := js.DeleteStream(stream); err != nil {
			return fmt.Errorf("delete %q stream: %w", stream, err)
		}
	}

	return bus.Disconnect(context.Background())
}
