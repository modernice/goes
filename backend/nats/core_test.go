//go:build nats

package nats_test

import (
	"testing"

	"github.com/modernice/goes/backend/nats"
	"github.com/modernice/goes/backend/testing/eventbustest"
	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/event"
)

func TestEventBus_Core(t *testing.T) {
	t.Run("Core", func(t *testing.T) {
		eventbustest.Run(t, newCoreEventBus)
		testEventBus(t, newCoreEventBus)
	})
}

func newCoreEventBus(enc codec.Encoding) event.Bus {
	return nats.NewEventBus(enc, nats.EatErrors(), nats.SubjectPrefix("core:"))
}
