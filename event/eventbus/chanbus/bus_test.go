package chanbus_test

import (
	"testing"

	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventbus/chanbus"
	"github.com/modernice/goes/event/eventbus/test"
)

func TestEventBus(t *testing.T) {
	test.EventBus(t, newBus)
}

func TestEventBus_Subscribe_multipleEvents(t *testing.T) {
	test.SubscribeMultipleEvents(t, newBus)
}

func TestEventBus_Subscribe_cancel(t *testing.T) {
	test.CancelSubscription(t, newBus)
}

func TestEventBus_Subscribe_canceledContext(t *testing.T) {
	test.SubscribeCanceledContext(t, newBus)
}

func TestEventBus_Publish_multipleEvents(t *testing.T) {
	test.PublishMultipleEvents(t, newBus)
}

func newBus(event.Encoder) event.Bus {
	return chanbus.New()
}
