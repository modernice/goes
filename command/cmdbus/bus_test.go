package cmdbus_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/modernice/goes/command"
	"github.com/modernice/goes/command/cmdbus"
	"github.com/modernice/goes/command/cmdbus/dispatch"
	"github.com/modernice/goes/command/cmdbus/report"
	"github.com/modernice/goes/command/done"
	"github.com/modernice/goes/command/encoding"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventbus/chanbus"
)

type mockPayload struct {
	A string
}

func TestBus_Dispatch(t *testing.T) {
	bus, _, _ := newBus()

	commands, errs, err := bus.Subscribe(context.Background(), "foo-cmd")
	if err != nil {
		t.Fatalf("failed to subscribe: %v", err)
	}

	load := mockPayload{A: "foo"}
	cmd := command.New("foo-cmd", load)

	dispatchErr := make(chan error)
	go func() {
		if err = bus.Dispatch(context.Background(), cmd); err != nil {
			dispatchErr <- fmt.Errorf("failed to dispatch: %w", err)
		}
	}()

	var ctx command.Context
	var ok bool
L:
	for {
		select {
		case err := <-dispatchErr:
			t.Fatal(err)
		case err, ok := <-errs:
			if ok {
				t.Fatal(err)
			}
		case ctx, ok = <-commands:
			if !ok {
				t.Fatal("Context channel shouldn't be closed!")
			}
			break L
		}
	}

	if ctx == nil {
		t.Fatalf("Context shouldn't be nil!")
	}

	assertEqualCommands(t, ctx.Command(), cmd)
}

func TestDispatch_Report(t *testing.T) {
	bus, _, _ := newBus()

	commands, errs, err := bus.Subscribe(context.Background(), "foo-cmd")
	if err != nil {
		t.Fatalf("failed to subscribe: %v", err)
	}

	cmd := command.New("foo-cmd", mockPayload{A: "foo"})
	var rep report.Report

	dispatchErr := make(chan error)
	go func() { dispatchErr <- bus.Dispatch(context.Background(), cmd, dispatch.Report(&rep)) }()

	var ctx command.Context
	var ok bool
	select {
	case err := <-dispatchErr:
		t.Fatalf("Dispatch shouldn't return yet! returned %q", err)
	case err, ok := <-errs:
		if ok {
			t.Fatal(err)
		}
		errs = nil
	case ctx, ok = <-commands:
		if !ok {
			t.Fatal("Context channel shouldn't be closed!")
		}
	}

	mockError := errors.New("mock error")
	dur := 3 * time.Second
	if err = ctx.MarkDone(ctx, done.WithError(mockError), done.WithRuntime(dur)); err != nil {
		t.Fatalf("mark done: %v", err)
	}

	var dispatchError error
	select {
	case <-time.After(300 * time.Millisecond):
		t.Fatalf("Dispatch not done after %s", 300*time.Millisecond)
	case dispatchError = <-dispatchErr:
	}

	if dispatchError == nil {
		t.Fatalf("Dispatch should return an error!")
	}

	var execError *cmdbus.ExecutionError
	if !errors.As(dispatchError, &execError) {
		t.Fatalf("Dispatch should return a %T error; got %T", execError, dispatchError)
	}

	if !strings.Contains(execError.Error(), mockError.Error()) {
		t.Fatalf("Dispatch should return an error that contains %q; got %q", mockError.Error(), execError.Error())
	}

	if rep.Command().ID() != cmd.ID() {
		t.Errorf("Report has wrong Command ID. want=%s got=%s", cmd.ID(), rep.Command().ID())
	}

	if rep.Runtime() != dur {
		t.Errorf("Report has wrong runtime. want=%s got=%s", dur, rep.Runtime())
	}
}

func TestSynchronous(t *testing.T) {
	bus, _, _ := newBus()

	commands, errs, err := bus.Subscribe(context.Background(), "foo-cmd")
	if err != nil {
		t.Fatalf("failed to subscribe: %v", err)
	}

	cmd := command.New("foo-cmd", mockPayload{A: "foo"})

	dispatchErr := make(chan error)
	dispatchTime := make(chan time.Time)
	go func() {
		dispatchErr <- bus.Dispatch(context.Background(), cmd, dispatch.Synchronous())
		dispatchTime <- time.Now()
	}()

	var ctx command.Context
	var ok bool
L:
	for {
		select {
		case err, ok := <-errs:
			if !ok {
				errs = nil
			}
			t.Fatal(err)
		case ctx, ok = <-commands:
			if !ok {
				t.Fatal("Context channel shouldn't be closed!")
			}
			break L
		}
	}

	mockError := errors.New("mock error")
	now := time.Now()
	if err = ctx.MarkDone(ctx, done.WithRuntime(3*time.Second), done.WithError(mockError)); err != nil {
		t.Fatalf("mark as done: %v", err)
	}

	dispatchError := <-dispatchErr
	dispatchedAt := <-dispatchTime

	if dispatchedAt.Before(now) || dispatchedAt.After(now.Add(3*time.Second)) {
		t.Fatalf("Dispatch should return at ~%s; returned at %s", now, dispatchedAt)
	}

	if dispatchError == nil {
		t.Fatalf("Dispatch should fail with an error; got %v", dispatchError)
	}

	if !strings.Contains(dispatchError.Error(), mockError.Error()) {
		t.Errorf("Dispatch should return an error that contains %q", mockError.Error())
	}
}

func newBus() (command.Bus, event.Bus, command.Encoder) {
	enc := encoding.NewGobEncoder()
	enc.Register("foo-cmd", func() command.Payload {
		return mockPayload{}
	})
	ebus := chanbus.New()
	return cmdbus.New(enc, ebus), ebus, enc
}

func assertEqualCommands(t *testing.T, cmd1, cmd2 command.Command) {
	if cmd1.Name() != cmd2.Name() {
		t.Errorf("Command Name mismatch: %q != %q", cmd1.Name(), cmd2.Name())
	}
	if cmd1.ID() != cmd2.ID() {
		t.Errorf("Command ID mismatch: %s != %s", cmd1.ID(), cmd2.ID())
	}
	if cmd1.AggregateName() != cmd2.AggregateName() {
		t.Errorf("Command AggregateName mismatch: %q != %q", cmd1.AggregateName(), cmd2.AggregateName())
	}
	if cmd1.AggregateID() != cmd2.AggregateID() {
		t.Errorf("Command AggregateID mismatch: %s != %s", cmd1.AggregateID(), cmd2.AggregateID())
	}
	if !reflect.DeepEqual(cmd1.Payload(), cmd2.Payload()) {
		t.Errorf("Command Payload mismatch: %#v != %#v", cmd1.Payload(), cmd2.Payload())
	}
}
