package eventbus_test

import (
	"testing"

	"github.com/modernice/goes/backend/testing/eventbustest"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventbus"
)

func TestChanbus(t *testing.T) {
	eventbustest.Run(t, func(e event.Encoder) event.Bus { return eventbus.New() })
}
