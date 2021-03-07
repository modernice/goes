// +build nats

package natsbus_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventbus/natsbus"
	"github.com/modernice/goes/event/eventbus/test"
	eventtest "github.com/modernice/goes/event/test"
)

func TestEventBus(t *testing.T) {
	testEventBus(t, "Core", newBus)
	test.EventBus(t, newBus)
}

func testEventBus(t *testing.T, name string, newBus test.EventBusFactory) {
	t.Run("Base", func(t *testing.T) {
		test.EventBus(t, newBus)
	})
	t.Run("SubscribeMultipleEvents", func(t *testing.T) {
		test.SubscribeMultipleEvents(t, newBus)
	})
	t.Run("SubscribeCancel", func(t *testing.T) {
		test.CancelSubscription(t, newBus)
	})
	t.Run("SubscribeCanceledContext", func(t *testing.T) {
		test.SubscribeCanceledContext(t, newBus)
	})
	t.Run("PublishMultipleEvents", func(t *testing.T) {
		test.PublishMultipleEvents(t, newBus)
	})
	t.Run("SubscribeConnect", func(t *testing.T) {
		testSubscribeConnect(t, func(e event.Encoder) event.Bus {
			return natsbus.New(e, natsbus.EatErrors())
		})
	})
	t.Run("PublishConnect", func(t *testing.T) {
		testPublishConnect(t, func(e event.Encoder) event.Bus {
			return natsbus.New(e, natsbus.EatErrors())
		})
	})
	t.Run("PublishEncodeError", func(t *testing.T) {
		testPublishEncodeError(t, func(e event.Encoder) event.Bus {
			return natsbus.New(e, natsbus.EatErrors())
		})
	})
}

func testSubscribeConnect(t *testing.T, newBus test.EventBusFactory) {
	org := os.Getenv("NATS_URL")
	if err := os.Setenv("NATS_URL", "what://abc:1234"); err != nil {
		t.Fatal(fmt.Errorf("set environment variable %q: %w", "NATS_URL", err))
	}

	defer func() {
		if err := os.Setenv("NATS_URL", org); err != nil {
			t.Fatal(fmt.Errorf("reset environment variable %q: %w", "NATS_URL", err))
		}
	}()

	bus := newBus(eventtest.NewEncoder())
	events, _, err := bus.Subscribe(context.Background(), "foo")

	if events != nil {
		t.Error(fmt.Errorf("events should be nil; got %#v", events))
	}

	if err == nil {
		t.Error(fmt.Errorf("err shouldn't be nil; got %#v", err))
	}
}

func testPublishConnect(t *testing.T, newBus test.EventBusFactory) {
	org := os.Getenv("NATS_URL")
	if err := os.Setenv("NATS_URL", "what://abc:1234"); err != nil {
		t.Fatal(fmt.Errorf("set environment variable %q: %w", "NATS_URL", err))
	}

	defer func() {
		if err := os.Setenv("NATS_URL", org); err != nil {
			t.Fatal(fmt.Errorf("reset environment variable %q: %w", "NATS_URL", err))
		}
	}()

	bus := newBus(eventtest.NewEncoder())
	err := bus.Publish(context.Background(), event.New("foo", eventtest.FooEventData{}))

	if err == nil {
		t.Error(fmt.Errorf("err shouldn't be nil; got %#v", err))
	}
}

func testPublishEncodeError(t *testing.T, newBus test.EventBusFactory) {
	bus := newBus(eventtest.NewEncoder())
	err := bus.Publish(context.Background(), event.New("xyz", eventtest.UnregisteredEventData{}))

	if err == nil {
		t.Fatal(fmt.Errorf("expected err not to be nil; got %v", err))
	}
}

func newBus(enc event.Encoder) event.Bus {
	return natsbus.New(enc, natsbus.EatErrors())
}
