//go:build nats

package nats_test

import (
	"context"
	"testing"

	"github.com/modernice/goes/backend/nats"
	"github.com/modernice/goes/backend/testing/eventbustest"
	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/event"
)

func TestEventBus_Core(t *testing.T) {
	t.Run("Plain", func(t *testing.T) {
		eventbustest.RunCore(t, newCoreEventBus, eventbustest.Cleanup(coreCleanup))
		eventbustest.RunWildcard(t, newCoreEventBus, eventbustest.Cleanup(coreCleanup))
		testEventBus(t, newCoreEventBus)
	})

	t.Run("Queue/LoadBalancer", func(t *testing.T) {
		eventbustest.RunCore(t, newQueueCoreEventBus, eventbustest.Cleanup(coreCleanup))
		eventbustest.RunWildcard(t, newQueueCoreEventBus, eventbustest.Cleanup(coreCleanup))
		testEventBus(t, newQueueCoreEventBus)
	})
}

func newCoreEventBus(enc codec.Encoding) event.Bus {
	return nats.NewEventBus(enc, nats.EatErrors(), nats.SubjectPrefix("core:"))
}

func newQueueCoreEventBus(enc codec.Encoding) event.Bus {
	return nats.NewEventBus(enc, nats.EatErrors(), nats.SubjectPrefix("core:"), nats.LoadBalancer("queue"))
}

func coreCleanup(bus *nats.EventBus) error {
	return bus.Disconnect(context.Background())
}
