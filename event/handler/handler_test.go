package handler_test

import (
	"context"
	"testing"
	"time"

	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventbus"
	"github.com/modernice/goes/event/eventstore"
	"github.com/modernice/goes/event/handler"
	"github.com/modernice/goes/event/test"
)

func TestHandler(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus := eventbus.New()
	h := handler.New(bus)

	fooHandled := make(chan event.Of[test.FooEventData])
	barHandled := make(chan event.Of[test.BarEventData])

	h.RegisterEventHandler("foo", func(evt event.Event) { fooHandled <- event.Cast[test.FooEventData](evt) })
	h.RegisterEventHandler("bar", func(evt event.Event) { barHandled <- event.Cast[test.BarEventData](evt) })

	errs, err := h.Run(ctx)
	if err != nil {
		t.Fatalf("Run() failed with %q", err)
	}

	go func() {
		for err := range errs {
			panic(err)
		}
	}()

	if err := bus.Publish(ctx, event.New("foo", test.FooEventData{}).Any()); err != nil {
		t.Fatalf("Publish() failed with %q", err)
	}

	select {
	case <-time.After(time.Second):
		t.Fatalf("foo event was not handled")
	case <-fooHandled:
	}

	if err := bus.Publish(ctx, event.New("bar", test.BarEventData{}).Any()); err != nil {
		t.Fatalf("Publish() failed with %q", err)
	}

	select {
	case <-time.After(time.Second):
		t.Fatalf("bar event was not handled")
	case <-barHandled:
	}
}

func TestWithStore(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus := eventbus.New()
	store := eventstore.New()
	h := handler.New(bus, handler.WithStore(store))

	fooHandled := make(chan event.Of[test.FooEventData])
	barHandled := make(chan event.Of[test.BarEventData])

	h.RegisterEventHandler("foo", func(evt event.Event) { fooHandled <- event.Cast[test.FooEventData](evt) })
	h.RegisterEventHandler("bar", func(evt event.Event) { barHandled <- event.Cast[test.BarEventData](evt) })

	if err := store.Insert(ctx, event.New("foo", test.FooEventData{}).Any()); err != nil {
		t.Fatalf("Insert() failed with %q", err)
	}

	errs, err := h.Run(ctx)
	if err != nil {
		t.Fatalf("Run() failed with %q", err)
	}

	go func() {
		for err := range errs {
			panic(err)
		}
	}()

	select {
	case <-time.After(time.Second):
		t.Fatalf("foo event was not handled")
	case <-fooHandled:
	}

	if err := bus.Publish(ctx, event.New("bar", test.BarEventData{}).Any()); err != nil {
		t.Fatalf("Publish() failed with %q", err)
	}

	select {
	case <-time.After(time.Second):
		t.Fatalf("bar event was not handled")
	case <-barHandled:
	}
}
