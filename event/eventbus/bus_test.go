package eventbus_test

import (
	"testing"

	"github.com/modernice/goes/backend/testing/eventbustest"
	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventbus"
)

func TestChanbus(t *testing.T) {
	eventbustest.RunCore(t, newBus)
	eventbustest.RunWildcard(t, newBus)
}

func newBus(codec.Encoding) event.Bus {
	return eventbus.New()
}
