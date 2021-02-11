package cmdbus_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
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

		if data.Config != dispatch.Configure() {
			t.Errorf("evt.Data().Config should be %v; got %v", dispatch.Configure(), data.Config)
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
	bus := cmdbus.New(enc, ebus)

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
	bus := cmdbus.New(enc, ebus, cmdbus.AssignTimeout(0))

	ctx, cancel := context.WithCancel(context.Background())

	commands1, err := bus.Handle(ctx, "foo")
	if err != nil {
		t.Fatalf("failed to register handler: %v", err)
	}

	commands2, err := bus.Handle(ctx, "foo")
	if err != nil {
		t.Fatalf("failed to register handler: %v", err)
	}

	cmd := command.New(
		"foo",
		mockPayload{A: true, B: "bar"},
	)
	if err := bus.Dispatch(context.Background(), cmd); err != nil {
		t.Fatalf("failed to dispatch command: %v", err)
	}

	var handleCount int64
	var wg sync.WaitGroup
	wg.Add(2)
	go countHandle(&wg, &handleCount, commands1)
	go countHandle(&wg, &handleCount, commands2)
	go func() {
		<-time.After(500 * time.Millisecond)
		cancel()
	}()
	wg.Wait()

	if handleCount != 1 {
		t.Fatalf(
			"exactly 1 handler should receive the command, but %d received it",
			handleCount,
		)
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

	cmd := command.New("foo", mockPayload{})
	errc := make(chan error, 1)
	go func() {
		if err := bus.Dispatch(context.Background(), cmd, dispatch.Synchronous()); err != nil {
			errc <- fmt.Errorf("failed to dispatch %q command: %v", "foo", err)
			return
		}
		close(errc)
	}()

	var cmdCtx command.Context
	select {
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("didn't receive command after %s", 500*time.Millisecond)
	case err := <-errc:
		if err != nil {
			t.Fatal(err)
		}
		t.Fatalf("bus.Dispatch should not have returned yet")
	case cmdCtx = <-commands:
	}

	if err := cmdCtx.Done(context.Background(), nil); err != nil {
		t.Fatalf("cmdCtx.Done() failed: %v", err)
	}

	var dispatchError error
	select {
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("didn't receive error after %s", 200*time.Millisecond)
	case dispatchError = <-errc:
	}

	if dispatchError != nil {
		t.Fatalf("dispatchError should be <nil>; got %v", dispatchError)
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

func TestRegister(t *testing.T) {
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

func countHandle(wg *sync.WaitGroup, count *int64, commands <-chan command.Context) {
	defer wg.Done()
	for range commands {
		atomic.AddInt64(count, 1)
	}
}