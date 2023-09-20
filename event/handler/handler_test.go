package handler_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventbus"
	"github.com/modernice/goes/event/eventstore"
	"github.com/modernice/goes/event/handler"
	"github.com/modernice/goes/event/query"
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

func TestStartup(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus := eventbus.New()
	store := eventstore.New()
	h := handler.New(bus, handler.Startup(store))

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

func TestStartupQuery(t *testing.T) {
	bus := eventbus.New()
	store := eventstore.New()

	h := handler.New(bus, handler.Startup(store), handler.StartupQuery(func(event.Query) event.Query {
		return query.New(query.Name("bar"))
	}))

	fooHandled := make(chan event.Of[test.FooEventData])
	barHandled := make(chan event.Of[test.BarEventData])

	h.RegisterEventHandler("foo", func(evt event.Event) { fooHandled <- event.Cast[test.FooEventData](evt) })
	h.RegisterEventHandler("bar", func(evt event.Event) { barHandled <- event.Cast[test.BarEventData](evt) })

	if err := store.Insert(
		context.Background(),
		event.New("foo", test.FooEventData{}).Any(),
		event.New("bar", test.BarEventData{}).Any(),
	); err != nil {
		t.Fatalf("Insert() failed with %q", err)
	}

	errs, err := h.Run(context.Background())
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
		t.Fatalf("bar event was not handled")
	case <-barHandled:
	}

	select {
	case <-time.After(50 * time.Millisecond):
	case <-fooHandled:
		t.Fatalf("foo event was handled")
	}
}

func TestStartup_withQuery_merges_names(t *testing.T) {
	bus := eventbus.New()
	store := eventstore.New()

	testID := uuid.New()

	h := handler.New(bus, handler.Startup(store, query.Name("bar")))

	fooHandled := make(chan event.Of[test.FooEventData])
	barHandled := make(chan event.Of[test.BarEventData])

	h.RegisterEventHandler("foo", func(evt event.Event) {
		t.Log("Handling foo event")
		fooHandled <- event.Cast[test.FooEventData](evt)
	})
	h.RegisterEventHandler("bar", func(evt event.Event) {
		t.Log("Handling bar event")
		barHandled <- event.Cast[test.BarEventData](evt)
	})

	t1 := time.Now().Add(time.Minute)
	t2 := t1.Add(time.Second)

	if err := store.Insert(
		context.Background(),
		event.New("foo", test.FooEventData{}).Any(),
		event.New("bar", test.BarEventData{}, event.Time(t2)).Any(),
		event.New("bar", test.BarEventData{}, event.Time(t1), event.ID(testID)).Any(),
	); err != nil {
		t.Fatalf("Insert() failed with %q", err)
	}

	errs, err := h.Run(context.Background())
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

	select {
	case <-time.After(time.Second):
		t.Fatalf("bar event was not handled #1")
	case evt := <-barHandled:
		if evt.ID() != testID {
			t.Fatalf("expected event ID %q; got %q", testID, evt.ID())
		}
	}

	select {
	case <-time.After(time.Second):
		t.Fatalf("bar event was not handled #2")
	case evt := <-barHandled:
		if evt.ID() == testID {
			t.Fatalf("expected event ID not to be %q; got %q", testID, evt.ID())
		}
	}
}

func TestStartup_withQuery_merges_ids(t *testing.T) {
	bus := eventbus.New()
	store := eventstore.New()

	testID := uuid.New()

	h := handler.New(bus, handler.Startup(store, query.ID(testID)))

	fooHandled := make(chan event.Of[test.FooEventData])
	barHandled := make(chan event.Of[test.BarEventData])

	h.RegisterEventHandler("foo", func(evt event.Event) {
		t.Log("Handling foo event")
		fooHandled <- event.Cast[test.FooEventData](evt)
	})
	h.RegisterEventHandler("bar", func(evt event.Event) {
		t.Log("Handling bar event")
		barHandled <- event.Cast[test.BarEventData](evt)
	})

	if err := store.Insert(
		context.Background(),
		event.New("foo", test.FooEventData{}).Any(),
		event.New("bar", test.BarEventData{}, event.ID(testID)).Any(),
	); err != nil {
		t.Fatalf("Insert() failed with %q", err)
	}

	errs, err := h.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() failed with %q", err)
	}

	go func() {
		for err := range errs {
			panic(err)
		}
	}()

	select {
	case <-time.After(50 * time.Millisecond):
	case <-fooHandled:
		t.Fatalf("foo event was handled")
	}

	select {
	case <-time.After(time.Second):
		t.Fatalf("bar event was not handled")
	case evt := <-barHandled:
		if evt.ID() != testID {
			t.Fatalf("expected event ID %q; got %q", testID, evt.ID())
		}
	}
}
