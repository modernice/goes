// Package eventbustest tests event bus implementations.
package eventbustest

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/test"
	"github.com/modernice/goes/internal/xtime"
	"golang.org/x/sync/errgroup"
)

// EventBusFactory creates an event.Bus from an event.Encoder.
type EventBusFactory func(event.Encoder) event.Bus

// Run tests all functions of the event bus.
func Run(t *testing.T, newBus EventBusFactory) {
	Core(t, newBus)
	SubscribeMultipleEvents(t, newBus)
	SubscribeCanceledContext(t, newBus)
	CancelSubscription(t, newBus)
	PublishMultipleEvents(t, newBus)
}

// Core tests the core functionality of an event.Bus.
func Core(t *testing.T, newBus EventBusFactory) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus := newBus(encoder())

	subErrors := make(chan error)

	// given 5 subscribers who listen for "foo" events
	subscribers := make([]<-chan event.Event, 5)
	for i := 0; i < 5; i++ {
		events, errs, err := bus.Subscribe(ctx, "foo")
		if err != nil {
			t.Fatal(fmt.Errorf("[%d] subscribe: %w", i, err))
		}
		subscribers[i] = events

		go func() {
			for err := range errs {
				subErrors <- err
			}
		}()
	}

	// when a "foo" event is published #1
	data := test.FooEventData{A: "foobar"}
	evt := event.New("foo", data)

	// for every subscriber ...
	group, _ := errgroup.WithContext(ctx)
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
		if err := bus.Publish(ctx, evt); err != nil {
			publishError <- fmt.Errorf("publish: %w", err)
		}
	}()

	err := make(chan error)
	go func() { err <- group.Wait() }()

	select {
	case err, ok := <-subErrors:
		if !ok {
			subErrors = nil
			break
		}
		t.Fatal(err)
	case err := <-publishError:
		t.Fatal(err)
	case err := <-err:
		if err != nil {
			t.Fatal(err)
		}
	}
}

// SubscribeMultipleEvents tests if an event.Bus supports subscribing to
// multiple Events at once.
func SubscribeMultipleEvents(t *testing.T, newBus EventBusFactory) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus := newBus(encoder())

	// given a subscriber who listens for "foo" and "bar" events
	events, _, err := bus.Subscribe(ctx, "foo", "bar")
	if err != nil {
		t.Fatal(fmt.Errorf("subscribe: %w", err))
	}

	// when a "foo" event is published
	dataA := test.FooEventData{A: "foobar"}
	evt := event.New("foo", dataA)
	if err = bus.Publish(ctx, evt); err != nil {
		t.Fatal(fmt.Errorf("publish: %w", err))
	}

	// a "foo" event should be received
	if err = expectEvent("foo", events); err != nil {
		t.Fatal(err)
	}

	// when a "bar" event is published
	dataB := test.BarEventData{A: "barbaz"}
	evt = event.New("bar", dataB)
	if err = bus.Publish(ctx, evt); err != nil {
		t.Fatal(fmt.Errorf("publish: %w", err))
	}

	// a "bar" event should be received
	if err = expectEvent("bar", events); err != nil {
		t.Fatal(err)
	}
}

// CancelSubscription tests if subscriptions are cancellable through the
// provided Context.
func CancelSubscription(t *testing.T, newBus EventBusFactory) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus := newBus(encoder())

	// when subscribed to "foo" events
	events, errs, err := bus.Subscribe(ctx, "foo")
	if err != nil {
		t.Fatal(fmt.Errorf("subscribe: %w", err))
	}

	// when 5 events are published
	want := make([]event.Event, 5)
	pubErr := make(chan error)
	var wg sync.WaitGroup
	wg.Add(5)
	go func() {
		for i := range want {
			defer wg.Done()
			want[i] = event.New("foo", test.FooEventData{})
			if err = bus.Publish(ctx, want[i]); err != nil {
				pubErr <- fmt.Errorf("publish: %w", err)
			}
			<-time.After(10 * time.Millisecond)
		}
	}()

	// 5 events should be received
	for i := 0; i < 5; i++ {
		select {
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("[%d] didn't receive Event after %s", i, 100*time.Millisecond)
		case err := <-pubErr:
			t.Fatal(err)
		case err, ok := <-errs:
			if !ok {
				errs = nil
				break
			}
			t.Fatal(err)
		case e, ok := <-events:
			if !ok {
				t.Fatalf("Event channel shouldn't be closed!")
			}
			var correct bool
			for _, evt := range want {
				if evt.ID() == e.ID() {
					correct = true
					break
				}
			}
			if !correct {
				t.Fatal("received wrong Event")
			}
		}
	}

	wg.Wait()

	// when the ctx is canceled
	cancel()
	<-time.After(50 * time.Millisecond)

	go func() {
		// and another event is published
		evt := event.New("foo", test.FooEventData{})
		if err = bus.Publish(context.Background(), evt); err != nil {
			pubErr <- fmt.Errorf("publish: %w", err)
		}
	}()

	// events should be closed
	select {
	case err := <-pubErr:
		t.Fatal(err)
	case evt, ok := <-events:
		if ok {
			t.Fatal(fmt.Errorf("event channel should be closed; got %v", evt))
		}
	case <-time.After(time.Second):
		t.Fatal("didn't receive from events channel after 1s", xtime.Now())
	}
}

// SubscribeCanceledContext tests the behaviour of an event.Bus when subscribing
// with an already canceled Context.
func SubscribeCanceledContext(t *testing.T, newBus EventBusFactory) {
	bus := newBus(encoder())

	// given a canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// when subscribing to "foo" events
	events, errs, err := bus.Subscribe(ctx, "foo")

	// some implementations may return an error, some may not
	_ = err

	go func() {
		for err := range errs {
			panic(err)
		}
	}()

	if events != nil {
		select {
		case <-time.After(time.Second):
			t.Errorf("event channel not closed after %v", time.Second)
		case _, ok := <-events:
			if ok {
				t.Errorf("event channel should be closed!")
			}
		}
	}

	if errs != nil {
		select {
		case <-time.After(time.Second):
			t.Errorf("error channel not closed after %v", time.Second)
		case _, ok := <-errs:
			if ok {
				t.Errorf("error channel should be closed!")
			}
		}
	}
}

// PublishMultipleEvents tests if an event.Store behaves correctly when
// publishing many Events at once.
func PublishMultipleEvents(t *testing.T, newBus EventBusFactory) {
	foo := event.New("foo", test.FooEventData{A: "foo"})
	bar := event.New("bar", test.BarEventData{A: "bar"})
	baz := event.New("baz", test.BazEventData{A: "baz"})

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
			bus := newBus(encoder())
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			events, errs, err := bus.Subscribe(ctx, tt.subscribe...)
			if err != nil {
				t.Fatal(fmt.Errorf("subscribe to %v: %w", tt.subscribe, err))
			}

			pubError := make(chan error)
			go func() {
				if err = bus.Publish(ctx, tt.publish...); err != nil {
					pubError <- fmt.Errorf("publish: %w", err)
				}
			}()

			var received []event.Event
			for len(received) < len(tt.want) {
				select {
				case <-time.After(100 * time.Millisecond):
					t.Fatal(fmt.Errorf("didn't receive event after 100ms"))
				case err := <-pubError:
					t.Fatal(err)
				case err, ok := <-errs:
					if !ok {
						errs = nil
						break
					}
					t.Fatal(err)
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
				case err := <-pubError:
					t.Fatal(err)
				case err, ok := <-errs:
					if !ok {
						errs = nil
						break
					}
					t.Fatal(err)
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

var (
	encMux sync.Mutex
	enc    event.Encoder
)

func encoder() event.Encoder {
	encMux.Lock()
	defer encMux.Unlock()
	if enc != nil {
		return enc
	}
	enc = test.NewEncoder()
	return enc
}
