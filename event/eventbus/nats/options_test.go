// +build nats

package nats

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventbus/test"
	"github.com/nats-io/nats.go"
)

func TestQueueGroupByFunc(t *testing.T) {
	// given an event bus with a queue group func
	bus := New(test.NewEncoder(), QueueGroupByFunc(func(eventName string) string {
		return fmt.Sprintf("bar.%s", eventName)
	}))

	// given 5 "foo" subscribers
	subs := make([]<-chan event.Event, 5)
	for i := range subs {
		events, err := bus.Subscribe(context.Background(), "foo")
		if err != nil {
			t.Fatal(fmt.Errorf("[%d] subscribe to %q events: %w", i, "foo", err))
		}
		subs[i] = events
	}

	// when a "foo" event is published
	evt := event.New("foo", test.FooEventData{})
	err := bus.Publish(context.Background(), evt)
	if err != nil {
		t.Fatal(fmt.Errorf("publish event %#v: %w", evt, err))
	}

	// only 1 subscriber should received the event
	receivedChan := make(chan event.Event, len(subs))
	var wg sync.WaitGroup
	wg.Add(len(subs))
	go func() {
		defer close(receivedChan)
		wg.Wait()
	}()
	for _, events := range subs {
		go func(events <-chan event.Event) {
			defer wg.Done()
			select {
			case evt := <-events:
				receivedChan <- evt
			case <-time.After(100 * time.Millisecond):
			}
		}(events)
	}
	wg.Wait()

	var received []event.Event
	for evt := range receivedChan {
		received = append(received, evt)
	}

	if len(received) != 1 {
		t.Fatal(fmt.Errorf("expected exactly 1 subscriber to receive an event; %d subscribers received it", len(received)))
	}

	if !event.Equal(received[0], evt) {
		t.Fatal(fmt.Errorf("received event doesn't match published event\npublished: %#v\n\nreceived: %#v", evt, received[0]))
	}
}

func TestQueueGroupByEvent(t *testing.T) {
	bus := New(test.NewEncoder(), QueueGroupByEvent())
	names := []string{"foo", "bar", "baz"}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			if queue := bus.queueFunc(name); queue != name {
				t.Fatal(fmt.Errorf("expected queueFunc to return %q; got %q", name, queue))
			}
		})
	}
}

func TestConnectWith(t *testing.T) {
	bus := New(test.NewEncoder(), ConnectWith(
		func(opts *nats.Options) error {
			opts.AllowReconnect = true
			return nil
		},
		func(opts *nats.Options) error {
			opts.MaxReconnect = 4
			return nil
		},
	))

	var opts nats.Options
	for _, opt := range bus.connectOpts {
		opt(&opts)
	}
	if !opts.AllowReconnect {
		t.Error(fmt.Errorf("expected AllowReconnect option to be %v; got %v", true, opts.AllowReconnect))
	}
	if opts.MaxReconnect != 4 {
		t.Error(fmt.Errorf("expected MaxReconnect option to be %v; got %v", 4, opts.MaxReconnect))
	}
}
