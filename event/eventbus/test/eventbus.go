// Package test tests event.Bus implementations.
package test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/encoding"
	"golang.org/x/sync/errgroup"
)

// EventBusFactory creates an event.Bus from an event.Encoder.
type EventBusFactory func(event.Encoder) event.Bus

// FooEventData is a testing event.Data.
type FooEventData struct{ A string }

// BarEventData is a testing event.Data.
type BarEventData struct{ A string }

// BazEventData is a testing event.Data.
type BazEventData struct{ A string }

// UnregisteredEventData is a testing event.Data that's not registered in the
// Encoder returned by NewEncoder.
type UnregisteredEventData struct{ A string }

// NewEncoder returns a "gob" event.Encoder with registered "foo", "bar" and
// "baz" events.
func NewEncoder() event.Encoder {
	enc := encoding.NewGobEncoder()
	enc.Register("foo", FooEventData{})
	enc.Register("bar", BarEventData{})
	enc.Register("baz", BazEventData{})
	return enc
}

// EventBus tests an event.Bus implementation.
func EventBus(t *testing.T, newBus EventBusFactory) {
	t.Run("basic test", func(t *testing.T) {
		testEventStore(t, newBus)
	})

	t.Run("subscribe to multiple event names", func(t *testing.T) {
		testSubscribeMultipleItems(t, newBus)
	})

	t.Run("cancel subscription", func(t *testing.T) {
		testCancelSubscription(t, newBus)
	})

	t.Run("subscribe with canceled context", func(t *testing.T) {
		testSubscribeCanceledContext(t, newBus)
	})

	t.Run("publish multiple events", func(t *testing.T) {
		testPublishMultipleEvents(t, newBus)
	})
}

func testEventStore(t *testing.T, newBus EventBusFactory) {
	bus := newBus(NewEncoder())

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
	data := FooEventData{A: "foobar"}
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

func testSubscribeMultipleItems(t *testing.T, newBus EventBusFactory) {
	bus := newBus(NewEncoder())

	// given a subscriber who listens for "foo" and "bar" events
	events, err := bus.Subscribe(context.Background(), "foo", "bar")
	if err != nil {
		t.Fatal(fmt.Errorf("subscribe: %w", err))
	}

	// when a "foo" event is published
	dataA := FooEventData{A: "foobar"}
	evt := event.New("foo", dataA)
	if err = bus.Publish(context.Background(), evt); err != nil {
		t.Fatal(fmt.Errorf("publish: %w", err))
	}

	// a "foo" event should be received
	if err = expectEvent("foo", events); err != nil {
		t.Fatal(err)
	}

	// when a "bar" event is published
	dataB := BarEventData{A: "barbaz"}
	evt = event.New("bar", dataB)
	if err = bus.Publish(context.Background(), evt); err != nil {
		t.Fatal(fmt.Errorf("publish: %w", err))
	}

	// a "bar" event should be received
	if err = expectEvent("bar", events); err != nil {
		t.Fatal(err)
	}
}

func testCancelSubscription(t *testing.T, newBus EventBusFactory) {
	bus := newBus(NewEncoder())

	ctx, cancel := context.WithCancel(context.Background())

	// when subscribed to "foo" events
	events, err := bus.Subscribe(ctx, "foo")
	if err != nil {
		t.Fatal(fmt.Errorf("subscribe: %w", err))
	}

	// when the ctx is canceled
	cancel()
	// wait 10ms to ensure cancellation has propagated
	<-time.After(10 * time.Millisecond)

	// when a "foo" event is published
	if err = bus.Publish(context.Background(), event.New("foo", FooEventData{})); err != nil {
		t.Fatal(fmt.Errorf("publish: %w", err))
	}

	// events should be closed
	select {
	case evt, ok := <-events:
		if ok {
			t.Fatal(fmt.Errorf("event channel should be closed; got %v", evt))
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("didn't receive from events channel after 100ms")
	}
}

func testSubscribeCanceledContext(t *testing.T, newBus EventBusFactory) {
	bus := newBus(NewEncoder())

	// given a canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// when subscribing to "foo" events
	events, err := bus.Subscribe(ctx, "foo")

	// it should fail with context.Canceled
	if !errors.Is(err, context.Canceled) {
		t.Error(fmt.Errorf("err should be context.Canceled; got %v", err))
	}

	// events should be nil
	if events != nil {
		t.Error(fmt.Errorf("events should be nil"))
	}
}

func testPublishMultipleEvents(t *testing.T, newBus EventBusFactory) {
	foo := event.New("foo", FooEventData{A: "foo"})
	bar := event.New("bar", BarEventData{A: "bar"})
	baz := event.New("baz", BazEventData{A: "baz"})

	tests := []struct {
		name      string
		subscribe []string
		publish   []event.Event
		want      []event.Event
	}{
		{
			name:      "subscribed to 1 event",
			subscribe: []string{"foo"},
			publish:   []event.Event{foo, bar},
			want:      []event.Event{foo},
		},
		{
			name:      "subscribed to all events",
			subscribe: []string{"foo", "bar"},
			publish:   []event.Event{foo, bar},
			want:      []event.Event{foo, bar},
		},
		{
			name:      "subscribed to even more events",
			subscribe: []string{"foo", "bar", "baz"},
			publish:   []event.Event{foo, bar},
			want:      []event.Event{foo, bar},
		},
		{
			name:      "subscribed to no events",
			subscribe: nil,
			publish:   []event.Event{foo, bar, baz},
			want:      nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bus := newBus(NewEncoder())
			ctx := context.Background()

			events, err := bus.Subscribe(ctx, tt.subscribe...)
			if err != nil {
				t.Fatal(fmt.Errorf("subscribe to %v: %w", tt.subscribe, err))
			}

			if err = bus.Publish(ctx, tt.publish...); err != nil {
				t.Fatal(fmt.Errorf("publish: %w", err))
			}

			var received []event.Event
			for len(received) < len(tt.want) {
				select {
				case <-time.After(100 * time.Millisecond):
					t.Fatal(fmt.Errorf("didn't receive event after 100ms"))
				case evt := <-events:
					received = append(received, evt)
				}
			}

			if len(tt.want) > 0 {
				// if we didn't expect any events check that events channel has
				// no extra events
				select {
				case evt, ok := <-events:
					if !ok {
						t.Fatal("events shouldn't be closed")
					}
					t.Fatal(fmt.Errorf("shouldn't have received another event; got %#v", evt))
				default:
				}
			}

			receivedMap := make(map[uuid.UUID]event.Event)
			for _, evt := range received {
				receivedMap[evt.ID()] = evt
			}

			for _, want := range tt.want {
				evt := receivedMap[want.ID()]
				if evt == nil {
					t.Fatal(fmt.Errorf("didn't receive event %v", want))
				}

				if !event.Equal(evt, want) {
					t.Fatal(fmt.Errorf("received event doesn't match published event\npublished: %#v\nreceived: %#v", want, evt))
				}
			}
		})
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
