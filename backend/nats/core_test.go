//go:build nats

package nats_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/modernice/goes/backend/nats"
	"github.com/modernice/goes/backend/testing/eventbustest"
	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/event"
)

func TestEventBus_Core(t *testing.T) {
	t.Run("Core", func(t *testing.T) {
		eventbustest.Run(t, newCoreEventBus, uuid.New)
		testEventBus(t, newCoreEventBus)
	})
}

func newCoreEventBus(enc codec.Encoding) event.Bus[uuid.UUID] {
	return nats.NewEventBus(uuid.New, enc, nats.EatErrors(), nats.SubjectPrefix("core:"))
}
