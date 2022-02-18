package eventbus_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/modernice/goes/backend/testing/eventbustest"
	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventbus"
)

func TestChanbus(t *testing.T) {
	eventbustest.Run(t, func(e codec.Encoding) event.Bus[uuid.UUID] { return eventbus.New[uuid.UUID]() }, uuid.New)
}
