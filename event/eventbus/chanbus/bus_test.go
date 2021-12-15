package chanbus_test

import (
	"testing"

	"github.com/modernice/goes/backend/testing/eventbustest"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventbus/chanbus"
)

func TestEventBus(t *testing.T) {
	eventbustest.Run(t, newBus)
}

func newBus(event.Encoder) event.Bus {
	return chanbus.New()
}
