//go:build nats

package nats_test

import (
	"testing"

	"github.com/modernice/goes/backend/nats"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventbus/test"
)

func TestEventBus_Core(t *testing.T) {
	testEventBus(t, "Core", newCoreEventBus)
	test.EventBus(t, newCoreEventBus)
}

func newCoreEventBus(enc event.Encoder) event.Bus {
	return nats.NewEventBus(enc, nats.EatErrors())
}
