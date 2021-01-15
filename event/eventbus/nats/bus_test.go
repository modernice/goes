// +build nats

package nats_test

import (
	"testing"

	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventbus/nats"
	"github.com/modernice/goes/event/eventbus/test"
)

func TestEventBus(t *testing.T) {
	test.EventBus(t, func(enc event.Encoder) event.Bus {
		return nats.New(enc)
	})
}
