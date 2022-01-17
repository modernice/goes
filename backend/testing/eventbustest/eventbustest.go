// Package eventbustest tests event bus implementations.
package eventbustest

import (
	"context"
	"testing"
	"time"

	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/test"
)

// EventBusFactory creates an event.Bus from an codec.Encoding.
type EventBusFactory func(codec.Encoding[any]) event.Bus

// Run tests all functions of the event bus.
func Run(t *testing.T, newBus EventBusFactory) {
	t.Run("Basic", func(t *testing.T) {
		Basic(t, newBus)
	})
	t.Run("SubscribeMultipleEvents", func(t *testing.T) {
		SubscribeMultipleEvents(t, newBus)
	})
	t.Run("SubscribeCanceledContext", func(t *testing.T) {
		SubscribeCanceledContext(t, newBus)
	})
	t.Run("CancelSubscription", func(t *testing.T) {
		CancelSubscription(t, newBus)
	})
	t.Run("PublishMultipleEvents", func(t *testing.T) {
		PublishMultipleEvents(t, newBus)
	})
}

// Basic tests the basic functionality of an event bus. The test is successful if
// multiple subscribers of the same event receive the event when it is published.
func Basic(t *testing.T, newBus EventBusFactory) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus := newBus(enc)

	// Given 10 subscribers of "foo" events
	subs := make([]Subscription, 10)
	for i := range subs {
		var err error
		if subs[i].events, subs[i].errs, err = bus.Subscribe(ctx, "foo"); err != nil {
			t.Fatalf("subscribe: %v [event=%v, iter=%d]", err, "foo", i)
		}
	}

	// When publishing a "bar" event, nothing should be received by the subscribers.
	ex := Expect(ctx)
	for _, sub := range subs {
		ex.Nothing(sub, 50*time.Millisecond)
	}

	if err := bus.Publish(ctx, event.New("bar", test.BarEventData{}).Any()); err != nil {
		t.Fatalf("publish event: %v [event=%v]", err, "bar")
	}

	ex.Apply(t)

	// When publishing a "foo" event, every subscriber should receive the event.
	ex = Expect(ctx)
	for _, sub := range subs {
		ex.Event(sub, 100*time.Millisecond, "foo")
	}

	if err := bus.Publish(ctx, event.New("foo", test.FooEventData{}).Any()); err != nil {
		t.Fatalf("publish event: %v [event=%v]", err, "foo")
	}

	ex.Apply(t)
}

func SubscribeMultipleEvents(t *testing.T, newBus EventBusFactory) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus := newBus(enc)

	// Given a subscriber of "foo", "bar" and "baz" events
	events, errs, err := bus.Subscribe(ctx, "foo", "bar", "baz")
	if err != nil {
		t.Fatalf("subscribe: %v [events=%v]", err, []string{"foo", "bar", "baz"})
	}
	sub := Subscription{events, errs}

	// When "foo" and "baz" events are published, the events should be received
	ex := Expect(ctx)
	ex.Events(sub, 100*time.Millisecond, "foo", "baz")

	evts := []event.Event[any]{
		event.New("foo", test.FooEventData{}).Any(),
		event.New("baz", test.BazEventData{}).Any(),
	}

	for _, evt := range evts {
		if err := bus.Publish(ctx, evt); err != nil {
			t.Fatalf("publish event: %v [event=%v]", err, evt.Name())
		}
	}

	ex.Apply(t)
}

func SubscribeCanceledContext(t *testing.T, newBus EventBusFactory) {
	// Given a canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	bus := newBus(enc)

	// Given a "foo" subscriber
	events, errs, err := bus.Subscribe(ctx, "foo")

	// Some implementations may return an error, some may not.
	if err != nil {
		return
	}

	sub := Subscription{events, errs}

	exCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ex := Expect(exCtx)
	ex.Closed(sub, 50*time.Millisecond)
	ex.Apply(t)
}

func CancelSubscription(t *testing.T, newBus EventBusFactory) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus := newBus(enc)

	// Given a "foo" subscriber
	events, errs, err := bus.Subscribe(ctx, "foo")
	if err != nil {
		t.Fatalf("subscribe: %v [event=%v]", err, "foo")
	}

	sub := Subscription{events, errs}

	// When publishing a "foo" event, the event should be received
	ex := Expect(ctx)
	ex.Event(sub, 50*time.Millisecond, "foo")

	if err := bus.Publish(ctx, event.New("foo", test.FooEventData{}).Any()); err != nil {
		t.Fatalf("publish event: %v [event=%v]", err, "foo")
	}

	ex.Apply(t)

	// When the subscription ctx is canceled
	cancel()

	// And another "foo" event is published, the event should not be received
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()

	ex = Expect(ctx)
	ex.Closed(sub, 50*time.Millisecond)

	if err := bus.Publish(ctx, event.New("foo", test.FooEventData{}).Any()); err != nil {
		t.Fatalf("publish event: %v [event=%v]", err, "foo")
	}

	ex.Apply(t)
}

func PublishMultipleEvents(t *testing.T, newBus EventBusFactory) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus := newBus(enc)

	// Given a "foo", "bar", "baz" subscriber
	sub := MustSub(bus.Subscribe(ctx, "foo", "bar", "baz"))

	// When publishing "foo", "baz" and "foobar" events, the "foo" and "baz"
	// events should be received.
	ex := Expect(ctx)
	ex.Events(sub, 500*time.Millisecond, "foo", "baz")

	evts := []event.Event[any]{
		event.New("foo", test.FooEventData{}).Any(),
		event.New("baz", test.BazEventData{}).Any(),
		event.New("foobar", test.FoobarEventData{}).Any(),
	}

	if err := bus.Publish(ctx, evts...); err != nil {
		t.Fatalf("publish events: %v [events=%v]", err, []string{"foo", "bar", "foobar"})
	}

	ex.Apply(t)
}

var enc = test.NewEncoder()
