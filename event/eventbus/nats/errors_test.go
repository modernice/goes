package nats

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/modernice/goes/event/eventbus/test"
)

func TestEventBus_Errors(t *testing.T) {
	bus := New(test.NewEncoder())
	errs := bus.Errors(context.Background())

	// wait for error handling to have started
	select {
	case <-time.After(100 * time.Millisecond):
		t.Fatal(fmt.Errorf("bus.errorHandlingStarted not closed after 100ms"))
	case _, ok := <-bus.errorHandlingStarted:
		if ok {
			t.Fatal(fmt.Errorf("bus.errorHandlingStarted should be closed"))
		}
	}

	// when an error happens
	err := errors.New("foo")
	bus.error(err)

	// that error should be received
	select {
	case <-time.After(time.Second):
		t.Fatal(fmt.Errorf("didn't receive error after 100ms"))
	case received := <-errs:
		if received != err {
			t.Fatal(fmt.Errorf("received wrong error\nexpected: %#v\n\ngot: %#v", received, err))
		}
	}

	// no other error should be received
	select {
	case received, ok := <-errs:
		if !ok {
			t.Fatal(fmt.Errorf("errs shouldn't be closed"))
		}
		t.Fatal(fmt.Errorf("shouldn't have received another error; got %#v", received))
	case <-time.After(50 * time.Millisecond):
	}
}

func TestEventBus_Errors_unsubscribe(t *testing.T) {
	bus := New(test.NewEncoder())

	// given a cancellable context
	ctx, cancel := context.WithCancel(context.Background())

	// when subscribed to errors
	errs := bus.Errors(ctx)

	// wait for error handling
	select {
	case <-time.After(100 * time.Millisecond):
		t.Fatal(fmt.Errorf("bus.errorHandlingStarted not closed after 100ms"))
	case <-bus.errorHandlingStarted:
	}

	// when an error happens
	err := errors.New("foo")
	bus.error(err)

	// the error should be received
	select {
	case <-time.After(500 * time.Millisecond):
		t.Fatal(fmt.Errorf("didn't receive from errs after 500ms"))
	case received := <-errs:
		if received != err {
			t.Fatal(fmt.Errorf("received wrong error\nexpected: %#v\n\ngot: %#v", err, received))
		}
	}

	// when the context is canceled
	cancel()

	// errs should be closed
	select {
	case <-time.After(time.Second):
		t.Fatal(fmt.Errorf("didn't receive from errs after 1s"))
	case received, ok := <-errs:
		if ok {
			t.Fatal(fmt.Errorf("errs should be closed"))
		}
		if received != nil {
			t.Fatal(fmt.Errorf("received error should be %#v; got %#v", error(nil), received))
		}
	}
}

func TestEventBus_Errors_canceledContext(t *testing.T) {
	// when subscribing to errors with a canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	bus := New(test.NewEncoder())
	errs := bus.Errors(ctx)

	// errs should be closed immediately
	select {
	case <-time.After(100 * time.Millisecond):
		t.Fatal(fmt.Errorf("didn't receive from errs after 100ms"))
	case err, ok := <-errs:
		if ok {
			t.Fatal(fmt.Errorf("errs should be closed"))
		}
		if err != nil {
			t.Fatal(fmt.Errorf("received error should be %#v; got %#v", error(nil), err))
		}
	}
}

func TestEventBus_error_noSubscribers(t *testing.T) {
	bus := New(test.NewEncoder())
	err := errors.New("foo")

	for i := 0; i < 5; i++ {
		done := make(chan struct{})
		go func() {
			bus.error(err)
			close(done)
		}()

		select {
		case <-time.After(100 * time.Millisecond):
			t.Fatal(fmt.Errorf("[%d] done not closed after 100ms", i))
		case <-done:
		}
	}
}
