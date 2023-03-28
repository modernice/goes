package cmdbus_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/command"
	"github.com/modernice/goes/command/cmdbus"
	"github.com/modernice/goes/command/cmdbus/dispatch"
	"github.com/modernice/goes/command/cmdbus/report"
	"github.com/modernice/goes/command/finish"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventbus"
	"github.com/modernice/goes/internal/testutil"
)

type mockPayload struct {
	A string
}

func TestBus_Dispatch(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	bus, _, _ := newBus(ctx)

	commands, errs, err := bus.Subscribe(ctx, "foo-cmd")
	if err != nil {
		t.Fatalf("failed to subscribe: %v", err)
	}

	load := mockPayload{A: "foo"}
	cmd := command.New("foo-cmd", load)

	dispatchErr := make(chan error)
	go func() {
		if err := bus.Dispatch(ctx, cmd.Any()); err != nil {
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus, _, _ := newBus(ctx, cmdbus.AssignTimeout(0))
	// pubBus, _, _ := newBusWith(ctx, ereg, ebus, cmdbus.AssignTimeout(0))

	commands, errs, err := bus.Subscribe(ctx, "foo-cmd")

	if err != nil {
		t.Fatalf("failed to subscribe: %v", err)
	}

	cmd := command.New("foo-cmd", mockPayload{A: "foo"})
	var rep report.Report

	dispatchErr := make(chan error)
	go func() { dispatchErr <- bus.Dispatch(ctx, cmd.Any(), dispatch.Report(&rep)) }()

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
	case <-time.After(time.Second):
		t.Fatalf("Dispatch not done after %s", time.Second)
	case dispatchError = <-dispatchErr:
	}

	if dispatchError == nil {
		t.Fatalf("Dispatch should return an error!")
	}

	var execError *cmdbus.ExecutionError[any]
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

func TestSynchronous(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	subBus, ebus, ereg := newBus(ctx)
	pubBus, _, _ := newBusWith(ctx, ereg, ebus)

	commands, errs, err := subBus.Subscribe(context.Background(), "foo-cmd")
	if err != nil {
		t.Fatalf("failed to subscribe: %v", err)
	}

	cmd := command.New("foo-cmd", mockPayload{A: "foo"})

	dispatchErr := make(chan error)
	dispatchDoneTime := make(chan time.Time)
	go func() {
		dispatchErr <- pubBus.Dispatch(context.Background(), cmd.Any(), dispatch.Sync())
		dispatchDoneTime <- time.Now()
	}()

	var cmdCtx command.Context
	var ok bool
L:
	for {
		select {
		case err, ok := <-errs:
			if !ok {
				errs = nil
			}
			t.Fatal(err)
		case cmdCtx, ok = <-commands:
			if !ok {
				t.Fatal("Context channel shouldn't be closed!")
			}
			break L
		}
	}

	mockError := errors.New("mock error")
	beforeFinish := time.Now()
	if err = cmdCtx.Finish(ctx, finish.WithRuntime(3*time.Second), finish.WithError(mockError)); err != nil {
		t.Fatalf("mark as done: %v", err)
	}

	dispatchError := <-dispatchErr
	dispatchedAt := <-dispatchDoneTime

	if dispatchedAt.Before(beforeFinish) {
		t.Fatalf("Dispatch should return after %v; returned at %v", beforeFinish, dispatchedAt)
	}

	if dispatchError == nil {
		t.Fatalf("Dispatch should fail with an error; got %v", dispatchError)
	}

	if !strings.Contains(dispatchError.Error(), mockError.Error()) {
		t.Errorf("Dispatch should return an error that contains %q\n%s", mockError.Error(), cmp.Diff(mockError.Error(), dispatchError.Error()))
	}
}

func TestAssignTimeout(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus, _, _ := newBus(ctx, cmdbus.AssignTimeout(500*time.Millisecond))

	cmd := command.New("foo-cmd", mockPayload{})

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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus, _, _ := newBus(ctx, cmdbus.AssignTimeout(0))

	cmd := command.New("foo-cmd", mockPayload{})

	dispatchErrc := make(chan error)
	go func() { dispatchErrc <- bus.Dispatch(context.Background(), cmd.Any()) }()

	select {
	case <-dispatchErrc:
		t.Fatalf("Dispatch should never return")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestReceiveTimeout(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus, _, _ := newBus(ctx, cmdbus.ReceiveTimeout(100*time.Millisecond))

	commands, errs, err := bus.Subscribe(ctx, "foo-cmd")
	if err != nil {
		t.Fatalf("failed to subscribe: %v", err)
	}

	newCmd := func() command.Command { return command.New("foo-cmd", mockPayload{}).Any() }
	dispatchErrc := make(chan error)

	go func() {
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
			if !errors.Is(err, cmdbus.ErrReceiveTimeout) {
				t.Fatal(err)
			}
		case _, ok := <-commands:
			if !ok {
				t.Fatalf("command channel should not be closed")
			}

			<-time.After(200 * time.Millisecond)
			count++
			if count == 2 {
				break L
			}
		}
	}

	select {
	case _, ok := <-commands:
		if !ok {
			t.Fatalf("command channel should not be closed")
		}
		count++
		t.Fatalf("command channel should only send 2 commands; got %d", count)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestReceiveTimeout_0(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus, _, _ := newBus(ctx, cmdbus.ReceiveTimeout(0))

	subCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	commands, errs, err := bus.Subscribe(subCtx, "foo-cmd")
	if err != nil {
		t.Fatalf("failed to subscribe: %v", err)
	}

	newCmd := func() command.Command { return command.New("foo-cmd", mockPayload{}).Any() }
	dispatchErrc := make(chan error)

	go func() {
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
				t.Fatalf("command channel should no be closed")
			}
			<-time.After(100 * time.Millisecond)
			count++
			if count == 3 {
				break L
			}
		}
	}

	select {
	case _, ok := <-commands:
		if !ok {
			t.Fatalf("command channel should not be closed")
		}
		count++
		t.Fatalf("command channel should only send 3 commands; got %d", count)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestBus_SingleBusReceivesEvent(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus1, ebus, creg := newBus(ctx, cmdbus.ReceiveTimeout(0))
	bus2, _, _ := newBusWith(ctx, creg, ebus, cmdbus.ReceiveTimeout(0))
	pubBus, _, _ := newBusWith(ctx, creg, ebus, cmdbus.AssignTimeout(0))

	commands1, errs1, err := bus1.Subscribe(ctx, "foo-cmd")
	if err != nil {
		t.Fatalf("failed to subscribe to bus1: %v", err)
	}

	commands2, errs2, err := bus2.Subscribe(ctx, "foo-cmd")
	if err != nil {
		t.Fatalf("failed to subscribe to bus2: %v", err)
	}

	newCmd := func() command.Command { return command.New("foo-cmd", mockPayload{}).Any() }
	dispatchError := make(chan error)

	go func() {
		if err := pubBus.Dispatch(ctx, newCmd(), dispatch.Sync()); err != nil {
			dispatchError <- err
		}
	}()

	var count int
	timeout := time.NewTimer(200 * time.Millisecond)
	defer timeout.Stop()
	for {
		select {
		case err := <-errs1:
			t.Fatalf("bus1: %v", err)
		case err := <-errs2:
			t.Fatalf("bus2: %v", err)
		case err := <-dispatchError:
			t.Fatalf("dispatch: %v", err)
		case <-commands1:
			count++
		case <-commands2:
			count++
		case <-timeout.C:
			if count != 1 {
				t.Fatalf("command should have been received by exactly 1 bus; received by %d", count)
			}
			return
		}
	}
}

func TestFilter(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	subBus, ebus, ereg := newBus(
		ctx,
		cmdbus.Filter(func(cmd command.Command) bool {
			return cmd.Name() != "foo-cmd"
		}),
	)

	commands, errs, err := subBus.Subscribe(ctx, "foo-cmd", "bar-cmd")
	if err != nil {
		t.Fatalf("failed to subscribe: %v", err)
	}
	go testutil.PanicOn(errs)
	go func() {
		for ctx := range commands {
			ctx.Finish(ctx)
		}
	}()

	pubBus, _, _ := newBusWith(ctx, ereg, ebus, cmdbus.AssignTimeout(500*time.Millisecond))

	fooCmd := command.New("foo-cmd", mockPayload{})
	if err := pubBus.Dispatch(ctx, fooCmd.Any(), dispatch.Sync()); !errors.Is(err, cmdbus.ErrAssignTimeout) {
		t.Fatalf("dispatch should have timed out with %q; got %q", cmdbus.ErrAssignTimeout, err)
	}

	barCmd := command.New("bar-cmd", mockPayload{})
	if err := pubBus.Dispatch(ctx, barCmd.Any(), dispatch.Sync()); err != nil {
		t.Fatalf("dispatch should not have failed: %v", err)
	}
}

func newBus(ctx context.Context, opts ...cmdbus.Option) (command.Bus, event.Bus, *codec.Registry) {
	enc := codec.New()
	codec.Register[mockPayload](enc, "foo-cmd")
	codec.Register[mockPayload](enc, "bar-cmd")
	ebus := eventbus.New()

	return newBusWith(ctx, enc, ebus, opts...)
}

func newBusWith(ctx context.Context, reg *codec.Registry, ebus event.Bus, opts ...cmdbus.Option) (command.Bus, event.Bus, *codec.Registry) {
	bus := cmdbus.New[int](reg, ebus, opts...)

	running := make(chan struct{})
	go func() {
		errs, err := bus.Run(ctx)
		if err != nil {
			panic(err)
		}

		close(running)

		for err := range errs {
			panic(err)
		}
	}()

	select {
	case <-ctx.Done():
		panic(ctx.Err())
	case <-running:
	}

	return bus, ebus, reg
}

func assertEqualCommands(t *testing.T, cmd1, cmd2 command.Command) {
	if cmd1.Name() != cmd2.Name() {
		t.Errorf("Command Name mismatch: %q != %q", cmd1.Name(), cmd2.Name())
	}
	if cmd1.ID() != cmd2.ID() {
		t.Errorf("Command ID mismatch: %s != %s", cmd1.ID(), cmd2.ID())
	}

	id1, name1 := cmd1.Aggregate().Split()
	id2, name2 := cmd2.Aggregate().Split()

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
