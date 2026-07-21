//go:build nats

package workflow_test

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	natsbackend "github.com/modernice/goes/backend/nats"
	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/event/eventstore"
	"github.com/modernice/goes/workflow"
	"github.com/nats-io/nats.go"
)

func TestService_CoreRecoveryFromStore(t *testing.T) {
	requireNATS(t, natsURL())

	reg := newRegistry()
	store := eventstore.New()
	cmdBus := newCommandBus(t, reg)
	defer cmdBus.Close(t)
	factory, def := newOrderWorkflowDefinition(time.Second)

	subjectPrefix := "workflow_core_" + randomHex() + ":"
	queue := "workflow-core-queue-" + randomHex()

	publisher := newCorePublisherBus(reg, subjectPrefix)
	defer cleanupNATSBus(t, publisher)

	appStore := eventstore.WithBus(store, publisher)

	orderID := uuid.New()
	if err := appStore.Insert(context.Background(), newOrderPlaced(orderID, uuid.New())); err != nil {
		t.Fatalf("insert placed event while workflow service is offline: %v", err)
	}

	bus := newCoreWorkflowBus(reg, subjectPrefix, queue)
	defer cleanupNATSBus(t, bus)

	svc := workflow.NewService(workflow.Config{
		Commands:            reg,
		EventStore:          store,
		EventBus:            bus,
		CommandBus:          cmdBus,
		DispatchInterval:    defaultDispatchInterval,
		TimerResolution:     defaultTimerResolution,
		TriggerReplayWindow: 24 * time.Hour,
	}, def)

	cancel, wait := runService(t, svc)
	defer func() {
		cancel()
		wait()
	}()

	awaitCommand(t, cmdBus, 1)
	awaitState(t, func() bool {
		w := loadWorkflow(t, store, factory, orderID)
		return w.Status() == workflow.StatusRunning && w.PlacedCount == 1
	})
}

func TestService_JetStreamRecoveryFromStore(t *testing.T) {
	requireJetStream(t, jetStreamURL())

	reg := newRegistry()
	store := eventstore.New()
	cmdBus := newCommandBus(t, reg)
	defer cmdBus.Close(t)
	factory, def := newOrderWorkflowDefinition(time.Second)

	subjectPrefix := "workflow_js_" + randomHex() + ":"
	queue := "workflow-queue-" + randomHex()
	durable := "workflow-durable-" + randomHex()
	stream := "workflow-js-" + randomHex()

	publisher := natsbackend.NewEventBus(
		reg,
		natsbackend.EatErrors(),
		natsbackend.Use(natsbackend.JetStream(natsbackend.StreamName(stream))),
		natsbackend.URL(jetStreamURL()),
		natsbackend.SubjectPrefix(subjectPrefix),
	)
	defer cleanupNATSBus(t, publisher, stream)
	appStore := eventstore.WithBus(store, publisher)

	orderID := uuid.New()
	if err := appStore.Insert(context.Background(), newOrderPlaced(orderID, uuid.New())); err != nil {
		t.Fatalf("insert placed event while workflow service is offline: %v", err)
	}

	bus := newJetStreamWorkflowBus(reg, subjectPrefix, queue, durable, stream)
	defer cleanupNATSBus(t, bus, stream)

	svc := workflow.NewService(workflow.Config{
		Commands:            reg,
		EventStore:          store,
		EventBus:            bus,
		CommandBus:          cmdBus,
		DispatchInterval:    defaultDispatchInterval,
		TimerResolution:     defaultTimerResolution,
		TriggerReplayWindow: 24 * time.Hour,
	}, def)

	cancel, wait := runService(t, svc)
	defer func() {
		cancel()
		wait()
	}()

	awaitCommand(t, cmdBus, 1)
	awaitState(t, func() bool {
		w := loadWorkflow(t, store, factory, orderID)
		return w.Status() == workflow.StatusRunning && w.PlacedCount == 1
	})
}

func newCorePublisherBus(reg codec.Encoding, subjectPrefix string) *natsbackend.EventBus {
	return natsbackend.NewEventBus(
		reg,
		natsbackend.EatErrors(),
		natsbackend.URL(natsURL()),
		natsbackend.SubjectPrefix(subjectPrefix),
	)
}

func newCoreWorkflowBus(reg codec.Encoding, subjectPrefix, queue string) *natsbackend.EventBus {
	return natsbackend.NewEventBus(
		reg,
		natsbackend.EatErrors(),
		natsbackend.URL(natsURL()),
		natsbackend.SubjectPrefix(subjectPrefix),
		natsbackend.LoadBalancer(queue),
	)
}

func newJetStreamWorkflowBus(reg codec.Encoding, subjectPrefix, queue, durable, stream string) *natsbackend.EventBus {
	return natsbackend.NewEventBus(
		reg,
		natsbackend.EatErrors(),
		natsbackend.Use(natsbackend.JetStream(natsbackend.StreamName(stream), natsbackend.Durable(durable))),
		natsbackend.URL(jetStreamURL()),
		natsbackend.SubjectPrefix(subjectPrefix),
		natsbackend.LoadBalancer(queue),
	)
}

func natsURL() string {
	if url := os.Getenv("NATS_URL"); url != "" {
		return url
	}
	return nats.DefaultURL
}

func jetStreamURL() string {
	if url := os.Getenv("JETSTREAM_URL"); url != "" {
		return url
	}
	return natsURL()
}

func requireNATS(t *testing.T, url string) {
	t.Helper()

	conn, err := nats.Connect(url, nats.Timeout(500*time.Millisecond))
	if err != nil {
		t.Skipf("NATS not available at %s: %v", url, err)
		return
	}
	defer conn.Close()
}

func requireJetStream(t *testing.T, url string) {
	t.Helper()

	conn, err := nats.Connect(url, nats.Timeout(500*time.Millisecond))
	if err != nil {
		t.Skipf("JetStream not available at %s: %v", url, err)
		return
	}
	defer conn.Close()

	if _, err := conn.JetStream(); err != nil {
		t.Skipf("JetStream not available at %s: %v", url, err)
	}
}

func cleanupNATSBus(t *testing.T, bus *natsbackend.EventBus, stream ...string) {
	t.Helper()
	if bus == nil || bus.Connection() == nil {
		return
	}

	js, err := bus.Connection().JetStream()
	if err == nil {
		for _, name := range stream {
			for consumer := range js.ConsumerNames(name) {
				if err := js.DeleteConsumer(name, consumer); err != nil && err != nats.ErrConsumerNotFound {
					t.Fatalf("delete %q consumer from %q: %v", consumer, name, err)
				}
			}
			if err := js.DeleteStream(name); err != nil && err != nats.ErrStreamNotFound {
				t.Fatalf("delete %q stream: %v", name, err)
			}
		}
	}

	if err := bus.Disconnect(context.Background()); err != nil {
		t.Fatalf("disconnect NATS bus: %v", err)
	}
}

func randomHex() string {
	buf := make([]byte, 8)
	rand.Read(buf)
	return fmt.Sprintf("%x", buf)
}
