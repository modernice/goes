//go:build nats

package nats_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/modernice/goes/backend/nats"
	"github.com/modernice/goes/backend/testing/eventbustest"
	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/event"
	eventtest "github.com/modernice/goes/event/test"
)

func testEventBus(t *testing.T, newBus eventbustest.EventBusFactory[uuid.UUID]) {
	t.Run("SubscribeConnect", func(t *testing.T) {
		testSubscribeConnect(t, func(e codec.Encoding) event.Bus[uuid.UUID] {
			return nats.NewEventBus(uuid.New, e, nats.EatErrors())
		})
	})
	t.Run("PublishConnect", func(t *testing.T) {
		testPublishConnect(t, func(e codec.Encoding) event.Bus[uuid.UUID] {
			return nats.NewEventBus(uuid.New, e, nats.EatErrors())
		})
	})
	t.Run("PublishEncodeError", func(t *testing.T) {
		testPublishEncodeError(t, func(e codec.Encoding) event.Bus[uuid.UUID] {
			return nats.NewEventBus(uuid.New, e, nats.EatErrors())
		})
	})
}

func testSubscribeConnect(t *testing.T, newBus eventbustest.EventBusFactory[uuid.UUID]) {
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

func testPublishConnect(t *testing.T, newBus eventbustest.EventBusFactory[uuid.UUID]) {
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
	err := bus.Publish(context.Background(), event.New(uuid.New(), "foo", eventtest.FooEventData{}).Any())

	if err == nil {
		t.Error(fmt.Errorf("err shouldn't be nil; got %#v", err))
	}
}

func testPublishEncodeError(t *testing.T, newBus eventbustest.EventBusFactory[uuid.UUID]) {
	bus := newBus(eventtest.NewEncoder())
	err := bus.Publish(context.Background(), event.New(uuid.New(), "xyz", eventtest.UnregisteredEventData{}).Any())

	if err == nil {
		t.Fatal(fmt.Errorf("expected err not to be nil; got %v", err))
	}
}
