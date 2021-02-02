package errbus_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/modernice/goes/internal/errbus"
)

func TestBus_Publish(t *testing.T) {
	bus := errbus.New()
	mockError := errors.New("foo")
	if err := bus.Publish(context.Background(), mockError); err != nil {
		t.Fatalf("failed to publish error: %v", err)
	}
}

func TestBus_Subscribe(t *testing.T) {
	bus := errbus.New()

	errs := bus.Subscribe(context.Background())

	mockError := errors.New("foo")
	if pubError := bus.Publish(context.Background(), mockError); pubError != nil {
		t.Fatalf("failed to publish error: %v", pubError)
	}

	select {
	case <-time.After(time.Second):
		t.Fatalf("didn't receive error after %s", time.Second)
	case err, ok := <-errs:
		if !ok {
			t.Fatalf("error channel should not be closed")
		}
		if err != mockError {
			t.Fatalf("received wrong error. want=%v got=%v", mockError, err)
		}
	}
}

func TestBus_Subscribe_cancelContext(t *testing.T) {
	bus := errbus.New()

	ctx, cancel := context.WithCancel(context.Background())
	errs := bus.Subscribe(ctx)
	cancel()

	if pubError := bus.Publish(context.Background(), errors.New("foo")); pubError != nil {
		t.Fatalf("failed to publish error: %v", pubError)
	}

	select {
	case err := <-errs:
		t.Fatalf("didn't expect to receive an error; got %#v", err)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestBus_Subscribe_multiple(t *testing.T) {
	bus := errbus.New()
	subs := make([]<-chan error, 5)
	for i := range subs {
		subs[i] = bus.Subscribe(context.Background())
	}

	mockError := errors.New("foo")
	if err := bus.Publish(context.Background(), mockError); err != nil {
		t.Fatalf("failed to publish error: %v", err)
	}

	for _, sub := range subs {
		select {
		case <-time.After(50 * time.Millisecond):
			t.Fatalf("didn't receive error after %s", 50*time.Millisecond)
		case err := <-sub:
			if err != mockError {
				t.Fatalf("received wrong error. want=%#v got=%#v", mockError, err)
			}
		}
	}
}
