package cmdbus_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/command"
	"github.com/modernice/goes/command/cmdbus"
	"github.com/modernice/goes/command/cmdbus/dispatch"
	"github.com/modernice/goes/command/cmdbus/report"
	"github.com/modernice/goes/command/finish"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventbus"
	"github.com/modernice/goes/internal/xtime"
)

type mockPayload struct {
	A string
}

func TestBus_Dispatch(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	bus, _, _ := newBus()

	commands, errs, err := bus.Subscribe(ctx, "foo-cmd")
	if err != nil {
		t.Fatalf("failed to subscribe: %v", err)
	}

	load := mockPayload{A: "foo"}
	cmd := command.New(uuid.New(), "foo-cmd", load)

	dispatchErr := make(chan error)
	go func() {
		if err = bus.Dispatch(ctx, cmd.Any()); err != nil {
			dispatchErr <- fmt.Errorf("failed to dispatch: %w", err)
		}
	}()

	var cmdCtx command.Context
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
		case cmdCtx, ok = <-commands:
			if !ok {
				t.Fatal("Context channel shouldn't be closed!")
			}
			break L
		}
	}

	if ctx == nil {
		t.Fatalf("Context shouldn't be nil!")
	}

	assertEqualCommands(t, cmdCtx, cmd.Any())
}

func TestBus_Dispatch_Report(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	bus, _, _ := newBus()

	commands, errs, err := bus.Subscribe(ctx, "foo-cmd")

	if err != nil {
		t.Fatalf("failed to subscribe: %v", err)
	}

	cmd := command.New(uuid.New(), "foo-cmd", mockPayload{A: "foo"})
	var rep report.Report[uuid.UUID]

	dispatchErr := make(chan error)
	go func() { dispatchErr <- bus.Dispatch(ctx, cmd.Any(), dispatch.Report[uuid.UUID](&rep)) }()

	var cmdCtx command.Context
	var ok bool
	select {
	case err := <-dispatchErr:
		t.Fatalf("Dispatch shouldn't return yet! returned %q", err)
	case err, ok := <-errs:
		if ok {
			t.Fatal(err)
		}
		errs = nil
	case cmdCtx, ok = <-commands:
		if !ok {
			t.Fatal("Context channel shouldn't be closed!")
		}
	}

	mockError := errors.New("mock error")
	dur := 3 * time.Second
	if err = cmdCtx.Finish(cmdCtx, finish.WithError(mockError), finish.WithRuntime(dur)); err != nil {
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

	var execError *cmdbus.ExecutionError[any, uuid.UUID]
	if !errors.As(dispatchError, &execError) {
		t.Fatalf("Dispatch should return a %T error; got %T", execError, dispatchError)
	}

	if !strings.Contains(execError.Error(), mockError.Error()) {
		t.Fatalf("Dispatch should return an error that contains %q; got %q", mockError.Error(), execError.Error())
	}

	if rep.Command.ID != cmd.ID() {
		t.Errorf("Report has wrong Command ID. want=%s got=%s", cmd.ID(), rep.Command.ID)
	}

	if rep.Runtime != dur {
		t.Errorf("Report has wrong runtime. want=%s got=%s", dur, rep.Runtime)
	}
}

func TestBus_Dispatch_cancel(t *testing.T) {
	bus, _, _ := newBus()

	cmd := command.New(uuid.New(), "foo-cmd", mockPayload{})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := bus.Dispatch(ctx, cmd.Any())

	if !errors.Is(err, cmdbus.ErrDispatchCanceled) {
		t.Fatalf("Dispatch should fail with %q; got %q", cmdbus.ErrDispatchCanceled, err)
	}
}

func TestSynchronous(t *testing.T) {
	bus, _, _ := newBus()

	commands, errs, err := bus.Subscribe(context.Background(), "foo-cmd")
	if err != nil {
		t.Fatalf("failed to subscribe: %v", err)
	}

	cmd := command.New(uuid.New(), "foo-cmd", mockPayload{A: "foo"})

	dispatchErr := make(chan error)
	dispatchTime := make(chan time.Time)
	go func() {
		dispatchErr <- bus.Dispatch(context.Background(), cmd.Any(), dispatch.Sync())
		dispatchTime <- xtime.Now()
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
	now := xtime.Now()
	if err = ctx.Finish(ctx, finish.WithRuntime(3*time.Second), finish.WithError(mockError)); err != nil {
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

func TestAssignTimeout(t *testing.T) {
	bus, _, _ := newBus(cmdbus.AssignTimeout(500 * time.Millisecond))

	cmd := command.New(uuid.New(), "foo-cmd", mockPayload{})

	dispatchErrc := make(chan error)
	go func() { dispatchErrc <- bus.Dispatch(context.Background(), cmd.Any()) }()

	var err error
	select {
	case <-time.After(time.Second):
		t.Fatalf("didn't receive error after %s", time.Second)
	case err = <-dispatchErrc:
	}

	if !errors.Is(err, cmdbus.ErrAssignTimeout) {
		t.Errorf("Dispatch should fail with %q; got %q", cmdbus.ErrAssignTimeout, err)
	}
}

func TestAssignTimeout_0(t *testing.T) {
	bus, _, _ := newBus(cmdbus.AssignTimeout(0))

	cmd := command.New(uuid.New(), "foo-cmd", mockPayload{})

	dispatchErrc := make(chan error)
	go func() { dispatchErrc <- bus.Dispatch(context.Background(), cmd.Any()) }()

	select {
	case <-dispatchErrc:
		t.Fatalf("Dispatch should never return")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestDrainTimeout(t *testing.T) {
	bus, _, _ := newBus(cmdbus.DrainTimeout(100 * time.Millisecond))

	subCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	commands, errs, err := bus.Subscribe(subCtx, "foo-cmd")
	if err != nil {
		t.Fatalf("failed to subscribe: %v", err)
	}

	newCmd := func() command.Command {
		return command.New[any](uuid.New(), "foo-cmd", mockPayload{})
	}
	dispatchErrc := make(chan error)

	go func() {
		defer cancel()
		for i := 0; i < 3; i++ {
			if err := bus.Dispatch(context.Background(), newCmd()); err != nil {
				dispatchErrc <- err
			}
		}
	}()

	var count int
L:
	for {
		select {
		case err, ok := <-errs:
			if !ok {
				errs = nil
				break
			}
			if !errors.Is(err, cmdbus.ErrDrainTimeout) {
				t.Fatal(err)
			}
		case _, ok := <-commands:
			if !ok {
				break L
			}
			<-time.After(200 * time.Millisecond)
			count++
		}
	}

	if count != 2 {
		t.Errorf("only %d Contexts should be received; got %d", 2, count)
	}
}

func TestDrainTimeout_0(t *testing.T) {
	bus, _, _ := newBus(cmdbus.DrainTimeout(0))

	subCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	commands, errs, err := bus.Subscribe(subCtx, "foo-cmd")
	if err != nil {
		t.Fatalf("failed to subscribe: %v", err)
	}

	newCmd := func() command.Command {
		return command.New[any](uuid.New(), "foo-cmd", mockPayload{})
	}
	dispatchErrc := make(chan error)

	go func() {
		defer cancel()
		if err := bus.Dispatch(context.Background(), newCmd()); err != nil {
			dispatchErrc <- err
		}
		if err := bus.Dispatch(context.Background(), newCmd()); err != nil {
			dispatchErrc <- err
		}
		if err := bus.Dispatch(context.Background(), newCmd()); err != nil {
			dispatchErrc <- err
		}
	}()

	var count int
L:
	for {
		select {
		case err, ok := <-errs:
			if ok {
				t.Fatal(err)
			}
			errs = nil
		case _, ok := <-commands:
			if !ok {
				break L
			}
			<-time.After(100 * time.Millisecond)
			count++
		}
	}

	if count != 3 {
		t.Errorf("all %d Contexts should be received; got %d", 3, count)
	}
}

func newBus(opts ...cmdbus.Option) (command.Bus[uuid.UUID], event.Bus[uuid.UUID], *codec.Registry) {
	enc := codec.Gob(codec.New())
	enc.GobRegister("foo-cmd", func() any {
		return mockPayload{}
	})
	ebus := eventbus.New[uuid.UUID]()
	return cmdbus.New(uuid.New, enc, ebus, opts...), ebus, enc.Registry
}

func assertEqualCommands(t *testing.T, cmd1, cmd2 command.Command) {
	if cmd1.Name() != cmd2.Name() {
		t.Errorf("Command Name mismatch: %q != %q", cmd1.Name(), cmd2.Name())
	}
	if cmd1.ID() != cmd2.ID() {
		t.Errorf("Command ID mismatch: %s != %s", cmd1.ID(), cmd2.ID())
	}

	id1, name1 := cmd1.Aggregate()
	id2, name2 := cmd2.Aggregate()

	if name1 != name2 {
		t.Errorf("Command AggregateName mismatch: %q != %q", name1, name2)
	}
	if id1 != id2 {
		t.Errorf("Command AggregateID mismatch: %s != %s", id1, id2)
	}
	if !reflect.DeepEqual(cmd1.Payload(), cmd2.Payload()) {
		t.Errorf("Command Payload mismatch: %#v != %#v", cmd1.Payload(), cmd2.Payload())
	}
}
