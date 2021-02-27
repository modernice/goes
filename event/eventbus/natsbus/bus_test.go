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
	test.EventBus(t, func(enc event.Encoder) event.Bus {
		return natsbus.New(enc)
	})
}

func TestEventBus_Subscribe_connect(t *testing.T) {
	org := os.Getenv("NATS_URL")
	if err := os.Setenv("NATS_URL", "what://abc:1234"); err != nil {
		t.Fatal(fmt.Errorf("set environment variable %q: %w", "NATS_URL", err))
	}

	defer func() {
		if err := os.Setenv("NATS_URL", org); err != nil {
			t.Fatal(fmt.Errorf("reset environment variable %q: %w", "NATS_URL", err))
		}
	}()

	bus := natsbus.New(eventtest.NewEncoder())
	events, err := bus.Subscribe(context.Background(), "foo")

	if events != nil {
		t.Error(fmt.Errorf("events should be nil; got %#v", events))
	}

	if err == nil {
		t.Error(fmt.Errorf("err shouldn't be nil; got %#v", err))
	}
}

func TestEventBus_Publish_connect(t *testing.T) {
	org := os.Getenv("NATS_URL")
	if err := os.Setenv("NATS_URL", "what://abc:1234"); err != nil {
		t.Fatal(fmt.Errorf("set environment variable %q: %w", "NATS_URL", err))
	}

	defer func() {
		if err := os.Setenv("NATS_URL", org); err != nil {
			t.Fatal(fmt.Errorf("reset environment variable %q: %w", "NATS_URL", err))
		}
	}()

	bus := natsbus.New(eventtest.NewEncoder())
	err := bus.Publish(context.Background(), event.New("foo", eventtest.FooEventData{}))

	if err == nil {
		t.Error(fmt.Errorf("err shouldn't be nil; got %#v", err))
	}
}

func TestEventBus_Publish_encodeError(t *testing.T) {
	bus := natsbus.New(eventtest.NewEncoder())
	err := bus.Publish(context.Background(), event.New("xyz", eventtest.UnregisteredEventData{}))

	if err == nil {
		t.Fatal(fmt.Errorf("expected err not to be nil; got %v", err))
	}
}
