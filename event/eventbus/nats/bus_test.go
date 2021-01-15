// +build nats

package nats_test

import (
	"context"
	"fmt"
	"os"
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

func TestEventBus_Subscribe_connect(t *testing.T) {
	org := os.Getenv("NATS_URI")
	if err := os.Setenv("NATS_URI", "what://abc:1234"); err != nil {
		t.Fatal(fmt.Errorf("set environment variable %q: %w", "NATS_URI", err))
	}

	defer func() {
		if err := os.Setenv("NATS_URI", org); err != nil {
			t.Fatal(fmt.Errorf("reset environment variable %q: %w", "NATS_URI", err))
		}
	}()

	bus := nats.New(test.NewEncoder())
	events, err := bus.Subscribe(context.Background(), "foo")

	if events != nil {
		t.Error(fmt.Errorf("events should be nil; got %#v", events))
	}

	if err == nil {
		t.Error(fmt.Errorf("err shouldn't be nil; got %#v", err))
	}
}

func TestEventBus_Publish_connect(t *testing.T) {
	org := os.Getenv("NATS_URI")
	if err := os.Setenv("NATS_URI", "what://abc:1234"); err != nil {
		t.Fatal(fmt.Errorf("set environment variable %q: %w", "NATS_URI", err))
	}

	defer func() {
		if err := os.Setenv("NATS_URI", org); err != nil {
			t.Fatal(fmt.Errorf("reset environment variable %q: %w", "NATS_URI", err))
		}
	}()

	bus := nats.New(test.NewEncoder())
	err := bus.Publish(context.Background(), event.New("foo", test.FooEventData{}))

	if err == nil {
		t.Error(fmt.Errorf("err shouldn't be nil; got %#v", err))
	}
}

func TestEventBus_Publish_encodeError(t *testing.T) {
	bus := nats.New(test.NewEncoder())
	err := bus.Publish(context.Background(), event.New("xyz", test.UnregisteredEventData{}))

	if err == nil {
		t.Fatal(fmt.Errorf("expected err not to be nil; got %v", err))
	}
}
