package command_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/command"
	"github.com/modernice/goes/command/cmdbus"
	"github.com/modernice/goes/command/cmdbus/dispatch"
	"github.com/modernice/goes/command/cmdbus/report"
	"github.com/modernice/goes/event/eventbus"
)

func TestHandler_Handle(t *testing.T) {
	enc := newEncoder()
	ebus := eventbus.New()
	subBus := cmdbus.New(enc, ebus)
	pubBus := cmdbus.New(enc, ebus)
	h := command.NewHandler[any](subBus)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	handled := make(chan command.Command)

	errs, err := h.Handle(ctx, "foo-cmd", func(ctx command.Context) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case handled <- ctx:
			return nil
		}
	})
	if err != nil {
		t.Fatalf("failed to register handler: %v", err)
	}

	cmd := command.New("foo-cmd", mockPayload{})
	go func() {
		if err := pubBus.Dispatch(ctx, cmd.Any()); err != nil {
			panic(fmt.Errorf("dispatch command: %w", err))
		}
	}()

	for {
		select {
		case err, ok := <-errs:
			if ok {
				t.Fatal(err)
				break
			}
			break
		case h := <-handled:
			if h.ID() != cmd.ID() || h.Name() != cmd.Name() || !reflect.DeepEqual(h.Payload(), cmd.Payload()) {
				t.Fatalf("handled Command differs from dispatched Command. want=%v got=%v", cmd, h)
			}
			return
		}
	}
}

func TestHandler_Handle_error(t *testing.T) {
	enc := newEncoder()
	ebus := eventbus.New()
	bus := cmdbus.New(enc, ebus)
	h := command.NewHandler[any](bus)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	mockError := errors.New("mock error")
	errs, err := h.Handle(ctx, "foo-cmd", func(ctx command.Context) error {
		return mockError
	})
	if err != nil {
		t.Fatalf("subscribe Command handler: %v", err)
	}

	cmd := command.New("foo-cmd", mockPayload{})
	go bus.Dispatch(ctx, cmd.Any())

	select {
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out")
	case err, ok := <-errs:
		if !ok {
			t.Fatal("error channel shouldn't be closed")
		}
		if !errors.Is(err, mockError) {
			t.Fatalf("expected %v error; got %v", mockError, err)
		}
	}
}

func TestHandler_Handle_finish(t *testing.T) {
	enc := newEncoder()
	ebus := eventbus.New()
	bus := cmdbus.New(enc, ebus)
	h := command.NewHandler[any](bus)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	mockError := errors.New("mock error")
	errs, err := h.Handle(ctx, "foo-cmd", func(ctx command.Context) error {
		return mockError
	})
	if err != nil {
		t.Fatalf("subscribe Command handler: %v", err)
	}

	cmd := command.New("foo-cmd", mockPayload{})

	dispatched := make(chan struct{})

	var rep report.Report
	go func() {
		bus.Dispatch(ctx, cmd.Any(), dispatch.Report(&rep))
		close(dispatched)
	}()

	timeout := time.NewTimer(time.Second)
	defer timeout.Stop()

L:
	for {
		select {
		case <-timeout.C:
			t.Fatal("timed out. was the Command Context finished?")
		case err := <-errs:
			if !errors.Is(err, mockError) {
				t.Fatalf("received wrong error. want=%v got=%v", mockError, err)
			}
		case <-dispatched:
			break L
		}
	}

	ref := cmd.Aggregate()

	wantCmd := report.Command{
		Name:          cmd.Name(),
		ID:            cmd.ID(),
		AggregateName: ref.Name,
		AggregateID:   ref.ID,
		Payload:       cmd.Payload(),
	}

	if !reflect.DeepEqual(rep.Command, wantCmd) {
		t.Fatalf("Report has wrong Command. want=%v got=%v", wantCmd, rep.Command)
	}

	execError, ok := cmdbus.ExecError[any](rep.Error)
	if !ok {
		t.Fatalf("Report error should be a %T; got %T\n\t%#v", execError, rep.Error, rep.Error)
	}

	if execError.Err.Error() != mockError.Error() {
		t.Fatalf("Report error should wrap %q; got %q", mockError, execError.Err)
	}
}

func newEncoder() codec.Encoding {
	reg := codec.Gob(codec.New())
	reg.GobRegister("foo-cmd", func() any { return mockPayload{} })
	return reg
}
