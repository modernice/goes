//go:build nats

package nats_test

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/modernice/goes/backend/nats"
	"github.com/modernice/goes/backend/testing/eventbustest"
	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/event"
	natsio "github.com/nats-io/nats.go"
)

func TestEventBus_JetStream(t *testing.T) {
	t.Run("Plain", jetStreamTest(newJetStreamBus))
	t.Run("Durable", jetStreamTest(newDurableJetStreamBus))
	t.Run("Queue", jetStreamTest(newQueueGroupJetStreamBus))
	t.Run("Durable+Queue", jetStreamTest(newDurableQueueGroupJetStreamBus))
}

func jetStreamTest(newBus func(string, codec.Encoding) event.Bus) func(t *testing.T) {
	return func(t *testing.T) {
		stream := "jetstream-" + randomHex()
		factory := func(enc codec.Encoding) event.Bus {
			return newBus(stream, enc)
		}
		cleanupFn := func(bus *nats.EventBus) error {
			return cleanup(bus, stream)
		}

		eventbustest.RunCore(t, factory, eventbustest.Cleanup(cleanupFn))
		eventbustest.RunWildcard(t, factory, eventbustest.Cleanup(cleanupFn))
		testEventBus(t, factory)
	}
}

var n int64

func newJetStreamBus(stream string, enc codec.Encoding) event.Bus {
	return nats.NewEventBus(
		enc,
		nats.EatErrors(),
		nats.Use(nats.JetStream(nats.StreamName(stream))),
		nats.URL(os.Getenv("JETSTREAM_URL")),
		nats.SubjectPrefix("jetstream:"),
	)
}

func newDurableJetStreamBus(stream string, enc codec.Encoding) event.Bus {
	return nats.NewEventBus(
		enc,
		nats.EatErrors(),
		nats.Use(nats.JetStream(nats.StreamName(stream), nats.Durable("durable"))),
		nats.URL(os.Getenv("JETSTREAM_URL")),
		nats.SubjectPrefix("jetstream_durable:"),
	)
}

func newQueueGroupJetStreamBus(stream string, enc codec.Encoding) event.Bus {
	return nats.NewEventBus(
		enc,
		nats.EatErrors(),
		nats.Use(nats.JetStream(nats.StreamName(stream))),
		nats.URL(os.Getenv("JETSTREAM_URL")),
		nats.LoadBalancer(randomQueue()),
		nats.SubjectPrefix("jetstream_queue:"),
	)
}

func newDurableQueueGroupJetStreamBus(stream string, enc codec.Encoding) event.Bus {
	return nats.NewEventBus(
		enc,
		nats.EatErrors(),
		nats.Use(nats.JetStream(nats.StreamName(stream), nats.Durable("durable_queue"))),
		nats.URL(os.Getenv("JETSTREAM_URL")),
		nats.LoadBalancer(randomQueue()),
		nats.SubjectPrefix("jetstream_durable_queue:"),
	)
}

func randomQueue() string {
	return randomHex()
}

func randomHex() string {
	buf := make([]byte, 8)
	rand.Read(buf)
	return fmt.Sprintf("%x", buf)
}

func cleanup(bus *nats.EventBus, stream string) error {
	js, err := bus.Connection().JetStream()
	if err != nil {
		return nil
	}

	for cons := range js.ConsumerNames(stream) {
		if err := js.DeleteConsumer(stream, cons); err != nil && !errors.Is(err, natsio.ErrConsumerNotFound) {
			return fmt.Errorf("delete %q consumer from %q: %w", cons, stream, err)
		}
	}

	if err := js.DeleteStream(stream); err != nil && !errors.Is(err, natsio.ErrStreamNotFound) {
		return fmt.Errorf("delete %q stream: %w", stream, err)
	}

	return bus.Disconnect(context.Background())
}
