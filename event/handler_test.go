package event_test

import (
	"context"
	"testing"
	"time"

	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventbus"
	"github.com/modernice/goes/internal/testutil"
)

func TestHandler(t *testing.T) {
	t.Parallel()

	var h event.Handler

	var (
		fooData = make(chan string)
		barData = make(chan int)
	)

	h.On("foo", func(e event.Event) {
		fooData <- e.Data().(string)
	})

	h.On("bar", func(e event.Event) {
		barData <- e.Data().(int)
	})

	bus := eventbus.New()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errs, err := h.Subscribe(ctx, bus)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	go testutil.PanicOn(errs)

	if err := bus.Publish(ctx, event.New("foo", "foo-data").Any()); err != nil {
		t.Fatalf("publish event: %v", err)
	}

	if data := <-fooData; data != "foo-data" {
		t.Errorf("expected fooData to be %q; got %q", "foo-data", data)
	}

	if err := bus.Publish(ctx, event.New("bar", 42).Any()); err != nil {
		t.Fatalf("publish event: %v", err)
	}

	if data := <-barData; data != 42 {
		t.Errorf("expected barData to be %d; got %d", 42, data)
	}
}

func TestHandler_sync(t *testing.T) {
	t.Parallel()

	var h event.Handler

	handled := make(chan struct{}, 2)

	h.On("foo", func(e event.Event) {
		time.Sleep(250 * time.Millisecond)
		handled <- struct{}{}
	})

	h.On("foo", func(e event.Event) {
		time.Sleep(250 * time.Millisecond)
		handled <- struct{}{}
	})

	bus := eventbus.New()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errs, err := h.Subscribe(ctx, bus)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	go testutil.PanicOn(errs)

	if err := bus.Publish(ctx, event.New("foo", "foo-data").Any()); err != nil {
		t.Fatalf("publish event: %v", err)
	}

	allHandled := make(chan struct{})
	go func() {
		<-handled
		<-handled
		close(allHandled)
	}()

	// If the handlers are blocking, the test will take more than 400ms.
	timeout := time.After(400 * time.Millisecond)

	select {
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	case <-allHandled:
		t.Fatal("expected handlers to block")
	case <-timeout:
	}
}

func TestHandler_async(t *testing.T) {
	t.Parallel()

	var h event.Handler
	h.Async(true)

	handled := make(chan struct{}, 2)

	h.On("foo", func(e event.Event) {
		time.Sleep(250 * time.Millisecond)
		handled <- struct{}{}
	})

	h.On("foo", func(e event.Event) {
		time.Sleep(250 * time.Millisecond)
		handled <- struct{}{}
	})

	bus := eventbus.New()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errs, err := h.Subscribe(ctx, bus)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	go testutil.PanicOn(errs)

	if err := bus.Publish(ctx, event.New("foo", "foo-data").Any()); err != nil {
		t.Fatalf("publish event: %v", err)
	}

	// If the handlers are non-blocking, the test should take aorund 250ms + some overhead.
	timeout := time.After(400 * time.Millisecond) // after 400ms, both handlers should have been called

	allHandled := make(chan struct{})
	go func() {
		<-handled
		<-handled
		close(allHandled)
	}()

	select {
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	case <-timeout:
		t.Fatal("expected handlers not to block")
	case <-allHandled:
	}
}

func TestOn(t *testing.T) {
	t.Parallel()

	var fooData string

	h := event.On("foo", func(e event.Of[string]) {
		fooData = e.Data()
	})

	bus := eventbus.New()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errs, err := h.Subscribe(ctx, bus)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	go testutil.PanicOn(errs)

	if err := bus.Publish(ctx, event.New("foo", "foo-data").Any()); err != nil {
		t.Fatalf("publish event: %v", err)
	}

	if fooData != "foo-data" {
		t.Errorf("expected fooData to be %q; got %q", "foo-data", fooData)
	}
}

func TestHandler_And(t *testing.T) {
	t.Parallel()

	handled := make(chan struct{})

	h1 := event.On("foo", func(e event.Of[string]) {
		handled <- struct{}{}
	})

	h2 := event.On("foo", func(e event.Of[string]) {
		handled <- struct{}{}
	})

	h3 := event.On("foo", func(e event.Of[string]) {
		handled <- struct{}{}
	})

	bus := eventbus.New()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errs, err := h1.And(h2, h3).Subscribe(ctx, bus)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	go testutil.PanicOn(errs)

	if err := bus.Publish(ctx, event.New("foo", "foo-data").Any()); err != nil {
		t.Fatalf("publish event: %v", err)
	}

	allHandled := make(chan struct{})
	go func() {
		<-handled
		<-handled
		<-handled
		close(allHandled)
	}()

	timeout := time.After(100 * time.Millisecond)

	select {
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	case <-timeout:
		t.Fatal("expected all three handlers to be called")
	case <-allHandled:
	}
}
