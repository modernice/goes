//go:build nats

package nats_test

import (
	"crypto/rand"
	"fmt"
	"os"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/modernice/goes/backend/nats"
	"github.com/modernice/goes/backend/testing/eventbustest"
	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/event"
)

func TestEventBus_JetStream(t *testing.T) {
	t.Run("JetStream", func(t *testing.T) {
		eventbustest.Run(t, newJetStreamBus, uuid.New)
		testEventBus(t, newJetStreamBus)
	})

	t.Run("JetStream+Durable", func(t *testing.T) {
		eventbustest.Run(t, newDurableJetStreamBus, uuid.New)
		testEventBus(t, newDurableJetStreamBus)
	})

	t.Run("JetStream+Queue", func(t *testing.T) {
		eventbustest.Run(t, newQueueGroupJetStreamBus, uuid.New)
		testEventBus(t, newQueueGroupJetStreamBus)
	})

	t.Run("JetStream+Durable+Queue", func(t *testing.T) {
		eventbustest.Run(t, newDurableQueueGroupJetStreamBus, uuid.New)
		testEventBus(t, newDurableQueueGroupJetStreamBus)
	})
}

var n int64

func newJetStreamBus(enc codec.Encoding) event.Bus[uuid.UUID] {
	return nats.NewEventBus(
		uuid.New,
		enc,
		nats.EatErrors(),
		nats.Use(nats.JetStream[uuid.UUID]()),
		nats.URL(os.Getenv("JETSTREAM_URL")),
		nats.SubjectPrefix("jetstream:"),
	)
}

func newDurableJetStreamBus(enc codec.Encoding) event.Bus[uuid.UUID] {
	return nats.NewEventBus(
		uuid.New,
		enc,
		nats.EatErrors(),
		nats.Use(nats.JetStream[uuid.UUID]()),
		nats.URL(os.Getenv("JETSTREAM_URL")),
		nats.DurableFunc(func(subject, _ string) string {
			num := atomic.AddInt64(&n, 1)
			return fmt.Sprintf("%s_%d", subject, num)
		}),
		nats.SubjectPrefix("jetstream_durable:"),
	)
}

func newQueueGroupJetStreamBus(enc codec.Encoding) event.Bus[uuid.UUID] {
	return nats.NewEventBus(
		uuid.New,
		enc,
		nats.EatErrors(),
		nats.Use(nats.JetStream[uuid.UUID]()),
		nats.URL(os.Getenv("JETSTREAM_URL")),
		nats.QueueGroup(randomQueue()),
		nats.SubjectPrefix("jetstream_queue:"),
	)
}

func newDurableQueueGroupJetStreamBus(enc codec.Encoding) event.Bus[uuid.UUID] {
	return nats.NewEventBus(
		uuid.New,
		enc,
		nats.EatErrors(),
		nats.Use(nats.JetStream[uuid.UUID]()),
		nats.URL(os.Getenv("JETSTREAM_URL")),
		nats.DurableFunc(func(subject, queue string) string {
			num := atomic.AddInt64(&n, 1)
			return fmt.Sprintf("%s_%s_%d", subject, queue, num)
		}),
		nats.QueueGroup(randomQueue()),
		nats.SubjectPrefix("jetstream_durable_queue:"),
	)
}

func randomQueue() string {
	buf := make([]byte, 8)
	rand.Read(buf)
	return fmt.Sprintf("%x", buf)
}
