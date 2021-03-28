package command_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/modernice/goes/command"
	"github.com/modernice/goes/command/cmdbus"
	"github.com/modernice/goes/command/encoding"
	"github.com/modernice/goes/event/eventbus/chanbus"
)

func TestHandler_Handle(t *testing.T) {
	ebus := chanbus.New()
	enc := newEncoder()
	bus := cmdbus.New(enc, ebus)
	h := command.NewHandler(bus)
	errs := h.Errors(context.Background())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mockError := errors.New("mock error")
	handled := make(chan struct{})
	err := h.Handle(ctx, "foo-cmd", func(command.Context) error {
		defer close(handled)
		return mockError
	})
	if err != nil {
		t.Fatal(err)
	}

	cmd := command.New("foo-cmd", mockPayload{})
	if err := bus.Dispatch(context.Background(), cmd); err != nil {
		t.Fatal(fmt.Errorf("dispatch: %w", err))
	}

	select {
	case <-time.After(time.Second):
		t.Fatal("Command not handled after 1s")
	case <-handled:
	}

	select {
	case <-time.After(time.Second):
		t.Fatal("didn't receive error after 1s")
	case err, ok := <-errs:
		if !ok {
			t.Fatal("error channel should not be closed")
		}
		if !errors.Is(err, mockError) {
			t.Fatalf("received wrong error. want=%q got=%q", mockError, err)
		}
	}
}

func newEncoder() command.Registry {
	r := encoding.NewGobEncoder()
	r.Register("foo-cmd", func() command.Payload { return mockPayload{} })
	return r
}
