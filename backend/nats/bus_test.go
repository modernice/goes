//go:build nats

package nats_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/modernice/goes/backend/nats"
	"github.com/modernice/goes/backend/testing/eventbustest"
	"github.com/modernice/goes/event"
	eventtest "github.com/modernice/goes/event/test"
)

func testEventBus(t *testing.T, newBus eventbustest.EventBusFactory) {
	t.Run("SubscribeConnect", func(t *testing.T) {
		testSubscribeConnect(t, func(e event.Encoder) event.Bus {
			return nats.NewEventBus(e, nats.EatErrors())
		})
	})
	t.Run("PublishConnect", func(t *testing.T) {
		testPublishConnect(t, func(e event.Encoder) event.Bus {
			return nats.NewEventBus(e, nats.EatErrors())
		})
	})
	t.Run("PublishEncodeError", func(t *testing.T) {
		testPublishEncodeError(t, func(e event.Encoder) event.Bus {
			return nats.NewEventBus(e, nats.EatErrors())
		})
	})
}

func testSubscribeConnect(t *testing.T, newBus eventbustest.EventBusFactory) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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
	events, errs, err := bus.Subscribe(ctx, "foo")
	go func() {
		for err := range errs {
			panic(err)
		}
	}()

	if events != nil {
		t.Error(fmt.Errorf("events should be nil; got %#v", events))
	}

	if err == nil {
		t.Error(fmt.Errorf("err shouldn't be nil; got %#v", err))
	}
}

func testPublishConnect(t *testing.T, newBus eventbustest.EventBusFactory) {
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

func testPublishEncodeError(t *testing.T, newBus eventbustest.EventBusFactory) {
	bus := newBus(eventtest.NewEncoder())
	err := bus.Publish(context.Background(), event.New("xyz", eventtest.UnregisteredEventData{}))

	if err == nil {
		t.Fatal(fmt.Errorf("expected err not to be nil; got %v", err))
	}
}
