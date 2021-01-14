package chanbus_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventbus/chanbus"
	"golang.org/x/sync/errgroup"
)

var _ event.Bus = chanbus.New()

type eventData struct {
	A string
}

func TestEventBus(t *testing.T) {
	bus := chanbus.New()

	// given 5 subscribers who listen for "foo" events
	subscribers := make([]<-chan event.Event, 5)
	for i := 0; i < 5; i++ {
		events, err := bus.Subscribe(context.Background(), "foo")
		if err != nil {
			t.Fatal(fmt.Errorf("[%d] subscribe: %w", i, err))
		}
		subscribers[i] = events
	}

	// when a "foo" event is published #1
	data := eventData{A: "foobar"}
	evt := event.New("foo", data)

	// for every subscriber ...
	group, _ := errgroup.WithContext(context.Background())
	for i, events := range subscribers {
		i := i
		events := events
		group.Go(func() error {
			if err := expectEvent("foo", events); err != nil {
				return fmt.Errorf("[%d] %w", i, err)
			}
			return nil
		})
	}

	// when a "foo" event is published #2
	publishError := make(chan error)
	go func() {
		if err := bus.Publish(context.Background(), evt); err != nil {
			publishError <- fmt.Errorf("publish: %w", err)
		}
	}()

	err := make(chan error)
	go func() { err <- group.Wait() }()

	select {
	case err := <-publishError:
		t.Fatal(err)
	case err := <-err:
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestEventBus_Subscribe_multipleNames(t *testing.T) {
	bus := chanbus.New()

	// given a subscriber who listens for "foo" and "bar" events
	events, err := bus.Subscribe(context.Background(), "foo", "bar")
	if err != nil {
		t.Fatal(fmt.Errorf("subscribe: %w", err))
	}

	// when a "foo" event is published
	data := eventData{A: "foobar"}
	evt := event.New("foo", data)
	if err = bus.Publish(context.Background(), evt); err != nil {
		t.Fatal(fmt.Errorf("publish: %w", err))
	}

	// a "foo" event should be received
	if err = expectEvent("foo", events); err != nil {
		t.Fatal(err)
	}

	// when a "bar" event is published
	data = eventData{A: "barbaz"}
	evt = event.New("bar", data)
	if err = bus.Publish(context.Background(), evt); err != nil {
		t.Fatal(fmt.Errorf("publish: %w", err))
	}

	// a "bar" event should be received
	if err = expectEvent("bar", events); err != nil {
		t.Fatal(err)
	}
}

func expectEvent(name string, events <-chan event.Event) error {
	select {
	case <-time.After(100 * time.Millisecond):
		return fmt.Errorf(`didn't receive "%s" event after 100ms`, name)
	case evt := <-events:
		if evt.Name() != name {
			return fmt.Errorf(`expected "%s" event; got "%s"`, name, evt.Name())
		}
	}
	return nil
}

func TestEventBus_Subscribe_cancel(t *testing.T) {
	bus := chanbus.New()

	ctx, cancel := context.WithCancel(context.Background())

	// when subscribed to "foo" events
	events, err := bus.Subscribe(ctx, "foo")
	if err != nil {
		t.Fatal(fmt.Errorf("subscribe: %w", err))
	}

	// when the ctx is canceled
	cancel()

	// when a "foo" event is published
	if err = bus.Publish(context.Background(), event.New("foo", eventData{})); err != nil {
		t.Fatal(fmt.Errorf("publish: %w", err))
	}

	// events should be closed
	select {
	case _, ok := <-events:
		if ok {
			t.Fatal("events channel should be closed")
		}
	case <-time.After(10 * time.Millisecond):
		t.Fatal("didn't receive from events channel after 10ms")
	}
}
