package nats

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventbus/test"
)

func TestError_Error(t *testing.T) {
	err := &Error{
		Err:   errors.New("foo"),
		Event: event.New("foo", test.FooEventData{}),
	}

	want := "nats eventbus: foo"
	if err.Error() != want {
		t.Fatal(fmt.Errorf("err.Error() should return %s; got %s", want, err.Error()))
	}
}

func TestError_Unwrap(t *testing.T) {
	err := &Error{
		Err: errors.New("foo"),
	}

	if is := errors.Is(err, err.Err); !is {
		t.Fatal(fmt.Errorf("expected errors.Is() to return true; got %v", is))
	}
}

func TestEventBus_Errors(t *testing.T) {
	bus := New(test.NewEncoder())

	errs := bus.Errors(context.Background())

	// should be a receive-only *Error channel
	if reflect.TypeOf(errs) != reflect.TypeOf(make(<-chan *Error)) {
		t.Fatal(fmt.Errorf(`channel should be of type %T; got %T`, make(<-chan *Error), errs))
	}

	// shouldn't be closed
	select {
	case err, ok := <-errs:
		if !ok {
			t.Fatal(fmt.Errorf("channel shouldn't be closed"))
		}
		t.Fatal(fmt.Errorf("shouldn't have received error; got %#v", err))
	case <-time.After(10 * time.Millisecond):
	}
}

func TestEventBus_Errors_cancel(t *testing.T) {
	bus := New(test.NewEncoder())

	ctx, cancel := context.WithCancel(context.Background())
	errs := bus.Errors(ctx)
	cancel()

	// errs should be closed
	select {
	case err, ok := <-errs:
		if ok {
			t.Fatal(fmt.Errorf("errs channel should be closed"))
		}
		if err != nil {
			t.Fatal(fmt.Errorf("shouldn't have received error; got %#v", err))
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal(fmt.Errorf("didn't receive from errs channel after 100ms"))
	}
}

func TestEventBus_Errors_receive(t *testing.T) {
	bus := New(test.NewEncoder())

	// given 3 error subscribers
	errChans := make([]<-chan *Error, 3)
	for i := range errChans {
		errChans[i] = bus.Errors(context.Background())
	}

	// when an error is pushed into the error queue
	err := &Error{Err: errors.New("foo"), Event: event.New("foo", test.FooEventData{})}
	go func() { bus.errs <- err }()

	// for every subscriber
	for _, errs := range errChans {
		// that error should be received
		select {
		case received := <-errs:
			if received != err {
				t.Error(fmt.Errorf("expected to receive %#v; got %#v", err, received))
			}
		case <-time.After(100 * time.Millisecond):
			t.Fatal("didn't receive error after 100ms")
		}

		// but no more
		select {
		case received := <-errs:
			t.Error(fmt.Errorf("shouldn't have received another error; got %#v", received))
		case <-time.After(10 * time.Millisecond):
		}
	}
}
