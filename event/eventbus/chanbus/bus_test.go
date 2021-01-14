package chanbus_test

import (
	"testing"

	"github.com/modernice/goes/event/eventbus/chanbus"
	"github.com/modernice/goes/event/eventbus/test"
)

func TestEventBus(t *testing.T) {
	test.EventBus(t, chanbus.New)
}
