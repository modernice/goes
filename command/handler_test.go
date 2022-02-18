package command_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/command"
	"github.com/modernice/goes/command/cmdbus"
	"github.com/modernice/goes/command/cmdbus/dispatch"
	"github.com/modernice/goes/command/cmdbus/report"
	"github.com/modernice/goes/event/eventbus"
)

func TestHandler_Handle(t *testing.T) {
	enc := newEncoder()
	ebus := eventbus.New[uuid.UUID]()
	bus := cmdbus.New(uuid.New, enc, ebus)
	h := command.NewHandler[any, uuid.UUID](bus)

	ctx, cancel := context.WithCancel(context.Background())
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

	dispatchError := make(chan error)

	cmd := command.New(uuid.New(), "foo-cmd", mockPayload{})
	go func() {
		if err := bus.Dispatch(ctx, cmd.Any()); err != nil {
			select {
			case <-ctx.Done():
			case dispatchError <- fmt.Errorf("dispatch Command: %w", err):
			}
		}
	}()

	select {
	case err, ok := <-errs:
		if !ok {
			t.Fatalf("error channel shouldn't be closed!")
		}
		t.Fatal(err)
	case h := <-handled:
		if h.ID() != cmd.ID() || h.Name() != cmd.Name() || !reflect.DeepEqual(h.Payload(), cmd.Payload()) {
			t.Fatalf("handled Command differs from dispatched Command. want=%v got=%v", cmd, h)
		}
	}
}

func TestHandler_Handle_error(t *testing.T) {
	enc := newEncoder()
	ebus := eventbus.New[uuid.UUID]()
	bus := cmdbus.New(uuid.New, enc, ebus)
	h := command.NewHandler[any, uuid.UUID](bus)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mockError := errors.New("mock error")
	errs, err := h.Handle(ctx, "foo-cmd", func(ctx command.Context) error {
		return mockError
	})
	if err != nil {
		t.Fatalf("subscribe Command handler: %v", err)
	}

	cmd := command.New(uuid.New(), "foo-cmd", mockPayload{})
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
	ebus := eventbus.New[uuid.UUID]()
	bus := cmdbus.New(uuid.New, enc, ebus)
	h := command.NewHandler[any, uuid.UUID](bus)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mockError := errors.New("mock error")
	errs, err := h.Handle(ctx, "foo-cmd", func(ctx command.Context) error {
		return mockError
	})
	if err != nil {
		t.Fatalf("subscribe Command handler: %v", err)
	}

	cmd := command.New(uuid.New(), "foo-cmd", mockPayload{})

	dispatched := make(chan struct{})

	var rep report.Report[uuid.UUID]
	go func() {
		bus.Dispatch(ctx, cmd.Any(), dispatch.Report[uuid.UUID](&rep))
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

	id, name := cmd.Aggregate()

	wantCmd := report.Command[uuid.UUID]{
		Name:          cmd.Name(),
		ID:            cmd.ID(),
		AggregateName: name,
		AggregateID:   id,
		Payload:       cmd.Payload(),
	}

	if !reflect.DeepEqual(rep.Command, wantCmd) {
		t.Fatalf("Report has wrong Command. want=%v got=%v\n%s", wantCmd, rep.Command, cmp.Diff(wantCmd, rep.Command))
	}

	execError, ok := cmdbus.ExecError[any, uuid.UUID](rep.Error)
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
