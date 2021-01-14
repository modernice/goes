// Package test tests event.Bus implementations.
package test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/encoding"
	"golang.org/x/sync/errgroup"
)

// EventBusFactory creates an event.Bus from an event.Encoder.
type EventBusFactory func(event.Encoder) event.Bus

type eventData struct {
	A string
}

// EventBus tests an event.Bus implementation.
func EventBus(t *testing.T, newBus EventBusFactory) {
	eventBus(t, newBus)
	subscribeMultipleNames(t, newBus)
	subscribeCancel(t, newBus)
	subscribeCanceledContext(t, newBus)
	publishMultipleEvents(t, newBus)
}

func eventBus(t *testing.T, newBus EventBusFactory) {
	bus := newBus(newEncoder())

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

func subscribeMultipleNames(t *testing.T, newBus EventBusFactory) {
	bus := newBus(newEncoder())

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

func subscribeCancel(t *testing.T, newBus EventBusFactory) {
	bus := newBus(newEncoder())

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
	case evt, ok := <-events:
		if ok {
			t.Fatal(fmt.Errorf("event channel should be closed; got %v", evt))
		}
	case <-time.After(10 * time.Millisecond):
		t.Fatal("didn't receive from events channel after 10ms")
	}
}

func subscribeCanceledContext(t *testing.T, newBus EventBusFactory) {
	bus := newBus(newEncoder())

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

func publishMultipleEvents(t *testing.T, newBus EventBusFactory) {
	foo := event.New("foo", eventData{A: "foo"})
	bar := event.New("bar", eventData{A: "bar"})
	baz := event.New("baz", eventData{A: "baz"})

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
			bus := newBus(newEncoder())
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

			// check that events channel has no extra events
			select {
			case evt := <-events:
				t.Fatal(fmt.Errorf("shouldn't have received another event; got %v", evt))
			default:
			}

			if !reflect.DeepEqual(received, tt.want) {
				t.Fatal(fmt.Errorf("expected events %v; got %v", tt.want, received))
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

func newEncoder() event.Encoder {
	enc := encoding.NewGobEncoder()
	enc.Register("foo", eventData{})
	return enc
}
