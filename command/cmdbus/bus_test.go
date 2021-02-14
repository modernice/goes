package cmdbus_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/modernice/goes/command"
	"github.com/modernice/goes/command/cmdbus"
	"github.com/modernice/goes/command/cmdbus/dispatch"
	"github.com/modernice/goes/command/cmdbus/report"
	"github.com/modernice/goes/command/done"
	"github.com/modernice/goes/command/encoding"
	mock_command "github.com/modernice/goes/command/mocks"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventbus/chanbus"
	mock_event "github.com/modernice/goes/event/mocks"
)

type mockPayload struct {
	A bool
	B string
}

func TestBus_Dispatch(t *testing.T) {
	ebus := chanbus.New()
	enc := encoding.NewGobEncoder()
	enc.Register("foo", mockPayload{})
	bus := cmdbus.New(enc, ebus, cmdbus.AssignTimeout(time.Second))

	var _ command.Bus = bus

	events, err := ebus.Subscribe(
		context.Background(),
		cmdbus.CommandDispatched,
	)

	if err != nil {
		t.Fatalf("subscribe to %q events: %v", "foo", err)
	}

	cmd := command.New(
		"foo",
		mockPayload{A: true, B: "foo"},
	)

	if err := bus.Dispatch(
		context.Background(),
		cmd,
	); !errors.Is(err, cmdbus.ErrAssignTimeout) {
		t.Fatalf("expected bus.Dispatch() to return a %v error; got %v", cmdbus.ErrAssignTimeout, err)
	}

	select {
	case <-time.After(time.Second):
		t.Fatalf("didn't receive event after %s", time.Second)
	case evt := <-events:
		if evt.Name() != "goes.command.dispatched" {
			t.Fatalf(
				"evt.Name should return %q; got %q",
				cmdbus.CommandDispatched,
				evt.Name(),
			)
		}

		data, ok := evt.Data().(cmdbus.CommandDispatchedData)
		if !ok {
			t.Fatalf(
				"evt.Data() should be type %T; got %T",
				cmdbus.CommandDispatchedData{},
				data,
			)
		}

		if data.Name != "foo" {
			t.Errorf("evt.Data().Name should be %q; got %q", "foo", data.Name)
		}

		if data.ID != cmd.ID() {
			t.Errorf("evt.Data().ID should be %s; got %s", cmd.ID(), data.ID)
		}

		if data.AggregateID != cmd.AggregateID() {
			t.Errorf(
				"evt.Data().AggregateID should be %q; got %q",
				cmd.AggregateID(),
				data.AggregateID,
			)
		}

		if data.AggregateName != cmd.AggregateName() {
			t.Errorf(
				"evt.Data().AggregateName should be %q; got %q",
				cmd.AggregateName(),
				data.AggregateName,
			)
		}

		pl, err := enc.Decode(data.Name, bytes.NewReader(data.Payload))
		if err != nil {
			t.Fatalf("failed to decode command payload: %v", err)
		}

		if !reflect.DeepEqual(pl, cmd.Payload()) {
			t.Errorf(
				"invalid command payload.\n\nwant: %#v\n\ngot: %#v\n\n",
				cmd.Payload(),
				data.Payload,
			)
		}
	}
}

func TestBus_Handle(t *testing.T) {
	enc := encoding.NewGobEncoder()
	enc.Register("foo", mockPayload{})
	ebus := chanbus.New()
	bus := cmdbus.New(enc, ebus)

	commands, err := bus.Handle(context.Background(), "foo", "bar", "baz")
	if err != nil {
		t.Fatalf("failed to register handler: %v", err)
	}

	commandRequested, err := ebus.Subscribe(context.Background(), cmdbus.CommandRequested)
	if err != nil {
		t.Fatalf("failed to subscribe %q event: %v", cmdbus.CommandRequested, err)
	}

	commandAssigned, err := ebus.Subscribe(context.Background(), cmdbus.CommandAssigned)
	if err != nil {
		t.Fatalf("failed to subscribe %q event: %v", cmdbus.CommandAssigned, err)
	}

	commandAccepted, err := ebus.Subscribe(context.Background(), cmdbus.CommandAccepted)
	if err != nil {
		t.Fatalf("failed to subscribe to %q event: %v", cmdbus.CommandAccepted, err)
	}

	// when a command is dispatched
	cmd := command.New("foo", mockPayload{A: true, B: "bar"})
	if err := bus.Dispatch(context.Background(), cmd); err != nil {
		t.Fatalf("failed to dispatch command: %v", err)
	}

	// the command should be received
	select {
	case <-time.After(time.Second):
		t.Fatalf("didn't receive command after %s", time.Second)
	case received, ok := <-commands:
		if !ok {
			t.Fatalf("command channel should not be closed")
		}

		if !reflect.DeepEqual(received.Command(), cmd) {
			t.Fatalf(
				"received command does not match dispatched command.\n\n"+
					"want: %#v\n\ngot: %#v\n\n",
				cmd,
				received,
			)
		}
	}

	// no other command should be received
	select {
	case cmd, ok := <-commands:
		if ok {
			t.Fatalf("didn't expect to receive a command; got %#v", cmd)
		}
		t.Fatalf("command channel should not be closed")
	case <-time.After(50 * time.Millisecond):
	}

	var handlerID uuid.UUID

	// the subscribed bus shoud publish a CommandRequested event
	select {
	case <-time.After(time.Second):
		t.Fatalf("didn't receive event after %s", time.Second)
	case evt := <-commandRequested:
		data, ok := evt.Data().(cmdbus.CommandRequestedData)
		if !ok {
			t.Fatalf("event data should be type %T; got %T", data, evt.Data())
		}

		if data.ID != cmd.ID() {
			t.Fatalf("data.ID should be %s; got %s", cmd.ID(), data.ID)
		}

		if data.HandlerID == uuid.Nil {
			t.Fatalf("data.HandlerID should not be %s", uuid.Nil)
		}

		handlerID = data.HandlerID
	}

	// the publishing bus should publish a CommandAssigned event
	select {
	case <-time.After(time.Second):
		t.Fatalf("didn't receive event after %s", time.Second)
	case evt := <-commandAssigned:
		data, ok := evt.Data().(cmdbus.CommandAssignedData)
		if !ok {
			t.Fatalf("event data should be type %T; got %T", data, evt.Data())
		}

		if data.HandlerID != handlerID {
			t.Fatalf("data.HandlerID should be %s; got %s", handlerID, data.HandlerID)
		}
	}

	// the subscribed bus shoud publish a CommandAccepted event
	select {
	case <-time.After(time.Second):
		t.Fatalf("didn't receive event after %s", time.Second)
	case evt := <-commandAccepted:
		data, ok := evt.Data().(cmdbus.CommandAcceptedData)
		if !ok {
			t.Fatalf("event data should be type %T; got %T", data, evt.Data())
		}

		if data.ID != cmd.ID() {
			t.Fatalf("data.ID should be %s; got %s", cmd.ID(), data.ID)
		}

		if data.HandlerID != handlerID {
			t.Fatalf("data.HandlerID should be %s; got %s", handlerID, data.HandlerID)
		}
	}
}

func TestBus_Handle_invalidEventData(t *testing.T) {
	enc := encoding.NewGobEncoder()
	enc.Register("foo", mockPayload{})
	ebus := chanbus.New()
	bus := cmdbus.New(enc, ebus)

	commands, err := bus.Handle(context.Background(), "foo")
	if err != nil {
		t.Fatalf("failed to register command handler: %v", err)
	}

	errs := bus.Errors(context.Background())

	evt := event.New(
		cmdbus.CommandDispatched,
		struct{ A string }{A: "foo"},
	)
	if err := ebus.Publish(context.Background(), evt); err != nil {
		t.Fatalf("failed to publish %q event: %v", evt.Name(), err)
	}

	select {
	case <-time.After(time.Second):
		t.Fatalf("didn't receive from channels after %s", time.Second)
	case cmd := <-commands:
		t.Fatalf("didn't expect to receive a command; got %#v", cmd)
	case err := <-errs:
		if err == nil {
			t.Fatalf("expected an error; got %#v", err)
		}
	}
}

func TestBus_Handle_decodeError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	enc := mock_command.NewMockEncoder(ctrl)

	ebus := chanbus.New()
	bus := cmdbus.New(enc, ebus, cmdbus.AssignTimeout(100*time.Millisecond))

	decodeError := errors.New("decode error")
	enc.EXPECT().
		Encode(gomock.Any(), gomock.Any()).
		DoAndReturn(func(w io.Writer, _ command.Payload) error {
			w.Write([]byte{})
			return nil
		})
	enc.EXPECT().Decode("foo", gomock.Any()).Return(nil, decodeError)

	_, err := bus.Handle(context.Background(), "foo")
	if err != nil {
		t.Fatalf("failed to register handler: %v", err)
	}

	errs := bus.Errors(context.Background())

	load := mockPayload{A: true, B: "bar"}
	cmd := command.New("foo", load)
	if err := bus.Dispatch(context.Background(), cmd); err == nil {
		t.Fatalf("bus.Dispatch() should return an error; got %v", err)
	}

	select {
	case <-time.After(time.Second):
		t.Fatalf("didn't receive error after %s", time.Second)
	case err := <-errs:
		if !errors.Is(err, decodeError) {
			t.Fatalf(
				"received wrong error type. want=%T got=%T",
				decodeError,
				err,
			)
		}
	}
}

func TestBus_Handle_exactlyOneHandler(t *testing.T) {
	enc := encoding.NewGobEncoder()
	enc.Register("foo", mockPayload{})
	ebus := chanbus.New()
	bus := cmdbus.New(enc, ebus)

	busErrors := bus.Errors(context.Background())

	subCtx, cancelSub := context.WithCancel(context.Background())

	commands, err := multiSubscribe(subCtx, bus, 3, "foo")
	if err != nil {
		t.Fatalf("mutli subscribe (%d): %v", 3, err)
	}

	var handleCount int64
	handleDone := make(chan struct{})
	go func() {
		defer close(handleDone)
		for cmd := range commands {
			atomic.AddInt64(&handleCount, 1)
			if err := cmd.Done(context.Background()); err != nil {
				panic(err)
			}
		}
	}()

	cmd := command.New("foo", mockPayload{})
	dispatchErrc := make(chan error)
	go func() {
		dispatchErrc <- bus.Dispatch(
			context.Background(),
			cmd,
			dispatch.Synchronous(), // make synchronous to avoid flakyness
		)
	}()

	select {
	case err := <-busErrors:
		t.Fatal(err)
	case err := <-dispatchErrc:
		if err != nil {
			t.Fatalf("dispatch: %v", err)
		}
	}

	cancelSub()

	select {
	case <-time.After(time.Second):
		t.Fatalf("didn't receive from handleDone channel after %s", time.Second)
	case <-handleDone:
	}

	if handleCount != 1 {
		t.Fatalf("exactly 1 handler should have received the command; got %d", handleCount)
	}
}

func TestBus_Dispatch_synchronous(t *testing.T) {
	enc := encoding.NewGobEncoder()
	enc.Register("foo", mockPayload{})
	ebus := chanbus.New()
	bus := cmdbus.New(enc, ebus, cmdbus.AssignTimeout(0))

	commands, err := bus.Handle(context.Background(), "foo")
	if err != nil {
		t.Fatalf("failed to subscribe to %q commands: %v", "foo", err)
	}

	errs := make(chan error, 2)
	go func() {
		for cmd := range commands {
			if err := cmd.Done(context.Background()); err != nil {
				errs <- err
			}
		}
	}()

	cmd := command.New("foo", mockPayload{})
	dispatched := make(chan struct{})
	go func() {
		if err := bus.Dispatch(context.Background(), cmd, dispatch.Synchronous()); err != nil {
			errs <- fmt.Errorf("failed to dispatch %q command: %v", cmd.Name(), err)
			return
		}
		close(dispatched)
	}()

	select {
	case err := <-errs:
		t.Fatal(err)
	case <-dispatched:
	}
}

func TestBus_Dispatch_synchronous_withError(t *testing.T) {
	enc := encoding.NewGobEncoder()
	enc.Register("foo", mockPayload{})
	ebus := chanbus.New()
	bus := cmdbus.New(enc, ebus)

	commands, err := bus.Handle(context.Background(), "foo")
	if err != nil {
		t.Fatalf("failed to subscribe to %q commands: %v", "foo", err)
	}

	errs := make(chan error, 2)
	mockError := errors.New("mock error")
	go func() {
		for cmd := range commands {
			if err := cmd.Done(context.Background(), done.WithError(mockError)); err != nil {
				errs <- err
			}
		}
	}()

	cmd := command.New("foo", mockPayload{})
	dispatchErrc := make(chan error)
	go func() { dispatchErrc <- bus.Dispatch(context.Background(), cmd, dispatch.Synchronous()) }()

	var dispatchError error
	select {
	case err := <-errs:
		t.Fatal(err)
	case err := <-dispatchErrc:
		dispatchError = err
	}

	execError, ok := cmdbus.ExecError(dispatchError)
	if !ok {
		t.Fatalf("bus.Dispatch() should return an error of type %T; got %T", execError, dispatchError)
	}

	if !reflect.DeepEqual(execError.Cmd, cmd) {
		t.Fatalf("execError.Cmd should be %v; got %v", cmd, execError.Cmd)
	}

	if execError.Err.Error() != mockError.Error() {
		t.Fatalf("execError.Err.Error() should return %q; got %q", mockError.Error(), execError.Err.Error())
	}
}

func TestBus_Dispatch_report(t *testing.T) {
	enc := encoding.NewGobEncoder()
	enc.Register("foo", mockPayload{})
	ebus := chanbus.New()
	bus := cmdbus.New(enc, ebus)

	commands, err := bus.Handle(context.Background(), "foo")
	if err != nil {
		t.Fatalf("failed to subscribe to %q commands: %v", "foo", err)
	}

	errs := make(chan error)
	mockError := errors.New("mock error")
	mockRuntime := time.Duration(rand.Intn(1000)) * time.Millisecond
	go func() {
		for cmd := range commands {
			if err := cmd.Done(
				context.Background(),
				done.WithRuntime(mockRuntime),
				done.WithError(mockError),
			); err != nil {
				errs <- fmt.Errorf("mark as done: %w", err)
			}
		}
	}()

	cmd := command.New("foo", mockPayload{})
	dispatchErrc := make(chan error)
	var rep report.Report
	go func() {
		dispatchErrc <- bus.Dispatch(
			context.Background(),
			cmd,
			dispatch.Synchronous(),
			dispatch.Report(&rep),
		)
	}()

	var dispatchError error
	select {
	case err := <-errs:
		t.Fatal(err)
	case err := <-dispatchErrc:
		dispatchError = err
	}

	if dispatchError == nil {
		t.Errorf("expected a dispatch error; got %v", dispatchErrc)
	}

	var execError *cmdbus.ExecutionError
	if !errors.As(dispatchError, &execError) {
		t.Errorf("dispatch error should unwrap to %T", execError)
	}

	if !reflect.DeepEqual(execError.Cmd, cmd) {
		t.Errorf("execError.Cmd should be %v; got %v", cmd, execError.Cmd)
	}

	if execError.Err.Error() != mockError.Error() {
		t.Errorf("execError.Err.Error() should return %q; got %q", mockError.Error(), execError.Err.Error())
	}

	if !reflect.DeepEqual(rep.Command(), cmd) {
		t.Errorf("rep.Command() should return %v; got %v", cmd, rep.Command())
	}

	if rep.Runtime() != mockRuntime {
		t.Errorf("rep.Runtime() should be %s; got %s", mockRuntime, rep.Runtime())
	}

	if rep.Error() == nil {
		t.Fatalf("rep.Error() should return an error; got %v", rep.Error())
	}

	if rep.Error().Error() != execError.Error() {
		t.Errorf("rep.Error().Error() should return %q; got %q", execError.Error(), rep.Error().Error())
	}
}

func TestAssignTimeout(t *testing.T) {
	enc := encoding.NewGobEncoder()
	ebus := chanbus.New()
	dur := 50 * time.Millisecond
	bus := cmdbus.New(enc, ebus, cmdbus.AssignTimeout(dur))
	cmd := command.New("foo", mockPayload{})

	errc := make(chan error)
	go func() {
		errc <- bus.Dispatch(context.Background(), cmd)
	}()

	select {
	case err := <-errc:
		if !errors.Is(err, cmdbus.ErrAssignTimeout) {
			t.Fatalf("expected error %q; got %q", cmdbus.ErrAssignTimeout, err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("didn't receive error after %s", 100*time.Millisecond)
	}
}

func TestDrainTimeout(t *testing.T) {
	enc := encoding.NewGobEncoder()
	ebus := chanbus.New()
	bus := cmdbus.New(enc, ebus, cmdbus.DrainTimeout(500*time.Millisecond), cmdbus.AssignTimeout(0))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	commands, err := bus.Handle(ctx, "foo")
	if err != nil {
		t.Fatalf("failed to subscribe to %q commands: %v", "foo", err)
	}

	cmd1 := command.New("foo", mockPayload{})
	cmd2 := command.New("foo", mockPayload{})
	errc := make(chan error, 2)
	go func() {
		if err := bus.Dispatch(context.Background(), cmd1); err != nil {
			errc <- fmt.Errorf("failed to dispatch %q command: %w", "foo", err)
		}
		if err := bus.Dispatch(context.Background(), cmd2); err != nil {
			errc <- fmt.Errorf("failed to dispatch %q command: %w", "foo", err)
		}
	}()

	select {
	case err := <-errc:
		t.Fatal(err)
	case <-time.After(50 * time.Millisecond):
		cancel()
	}

	select {
	case err := <-errc:
		t.Fatal(err)
	case <-time.After(time.Second):
	}

	<-commands

	select {
	case <-time.After(50 * time.Millisecond):
		t.Fatalf("commands channel not closed after %s", 50*time.Millisecond)
	case _, ok := <-commands:
		if ok {
			t.Fatalf("commands channel should be closed")
		}
	}
}

func TestRegisterEvents(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	reg := mock_event.NewMockRegistry(ctrl)
	reg.EXPECT().Register(cmdbus.CommandDispatched, cmdbus.CommandDispatchedData{})
	reg.EXPECT().Register(cmdbus.CommandRequested, cmdbus.CommandRequestedData{})
	reg.EXPECT().Register(cmdbus.CommandAssigned, cmdbus.CommandAssignedData{})
	reg.EXPECT().Register(cmdbus.CommandAccepted, cmdbus.CommandAcceptedData{})
	reg.EXPECT().Register(cmdbus.CommandExecuted, cmdbus.CommandExecutedData{})

	cmdbus.RegisterEvents(reg)
}

func multiSubscribe(ctx context.Context, bus command.Bus, count int, names ...string) (<-chan command.Context, error) {
	commands := make(chan command.Context)
	var wg sync.WaitGroup
	wg.Add(count)
	for i := 0; i < count; i++ {
		cmds, err := bus.Handle(ctx, names...)
		if err != nil {
			return nil, fmt.Errorf("subscribe to %q events: %w", names, err)
		}
		go func() {
			defer wg.Done()
			for cmd := range cmds {
				commands <- cmd
			}
		}()
	}
	go func() {
		wg.Wait()
		close(commands)
	}()
	return commands, nil
}
