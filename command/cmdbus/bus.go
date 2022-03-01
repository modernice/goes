// Package cmdbus provides a distributed & event-driven Command Bus.
package cmdbus

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/command"
	"github.com/modernice/goes/command/cmdbus/dispatch"
	"github.com/modernice/goes/command/cmdbus/report"
	"github.com/modernice/goes/command/finish"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/helper/streams"
	"github.com/modernice/goes/internal/concurrent"
)

var _ command.Bus = (*Bus)(nil)

const (
	// DefaultAssignTimeout is the default timeout for assigning a command to a
	// handler. If a command is not assigned to a handler within this timeout,
	// the command bus returns an error that unwraps to ErrAssignTimeout.
	// The default timeout is 5s. A zero Duration means no timeout.
	DefaultAssignTimeout = 5 * time.Second

	// DefaultReceiveTimeout is the default timeout for receiving commands from
	// the command bus. If the command is not received within this timeout, the
	// command bus returns an error that unwraps to ErrReceiveTimeout.
	// The default timeout is 10s. A zero Duration means no timeout.
	DefaultReceiveTimeout = 10 * time.Second
)

var (
	// ErrAssignTimeout is returned by a Bus when it fails to assign a Command
	// to a Handler before a given deadline.
	ErrAssignTimeout = errors.New("failed to assign command because of timeout")

	// ErrReceiveTimeout is emitted by a Bus when the DrainTimeout is exceeded
	// when receiving remaining Commands from a canceled Command subscription.
	ErrReceiveTimeout = errors.New("command dropped because of receive timeout")

	// Deprecated: Use ErrReceiveTimeout instead.
	ErrDrainTimeout = ErrReceiveTimeout

	// ErrDispatchCanceled is returned by a Bus when the dispatch was canceled
	// by the provided Context.
	ErrDispatchCanceled = errors.New("dispatch canceled")

	// // ErrNotRunning is returned when trying to dispatch or subscribe to a
	// // command before the command bus has been started.
	// ErrNotRunning = errors.New("command bus is not running")

	// ErrAlreadySubscribed is returned when trying to subscribe to the same
	// commands more than once within a single command bus.
	ErrSubscribed = errors.New("already subscribed to command")
)

// Bus is an event-driven Command Bus.
type Bus struct {
	*event.Handler

	mux           sync.RWMutex
	subscriptions map[string]*subscription
	requested     map[uuid.UUID]command.Cmd[any]
	dispatched    map[uuid.UUID]dispatcher
	assigned      map[uuid.UUID]dispatcher
	pending       map[uuid.UUID]dispatcher

	assignTimeout  time.Duration
	receiveTimeout time.Duration

	enc       codec.Encoding
	bus       event.Bus
	handlerID uuid.UUID

	errs chan error
	fail func(error)
}

type subscription struct {
	commands chan command.Context
	errs     chan error
}

type dispatcher struct {
	cmd      command.Command
	cfg      command.DispatchConfig
	accepted chan struct{}
	received chan struct{}
	out      chan error
}

// Option is a Command Bus option.
type Option func(*Bus)

// // Debug enables verbose logging for debugging purposes. Optional id may be
// // specified to annotate debug output.
// func Debug(id string) Option {
// 	return func(b *Bus) {
// 		b.debug = true
// 		b.debugID = id
// 	}
// }

// AssignTimeout returns an Option that configures the timeout when assigning a
// Command to a Handler. A zero Duration means no timeout.
//
// A zero Duration means no timeout. The default timeout is 5s.
func AssignTimeout(dur time.Duration) Option {
	return func(b *Bus) {
		b.assignTimeout = dur
	}
}

// ReceiveTimeout returns an Option that configures the timeout for receiving a
// command context from the command bus. If the command is not received from the
// returned channel within the configured timeout, the command is dropped.
//
// A zero Duration means no timeout. The default timeout is 10s.
func ReceiveTimeout(dur time.Duration) Option {
	return func(b *Bus) {
		b.receiveTimeout = dur
	}
}

// Deprecated: Use ReceiveTimeout instead.
func DrainTimeout(dur time.Duration) Option {
	return ReceiveTimeout(dur)
}

// New returns an event-driven command bus.
func New(enc codec.Encoding, events event.Bus, opts ...Option) *Bus {
	b := &Bus{
		Handler:        event.NewHandler(events),
		subscriptions:  make(map[string]*subscription),
		requested:      make(map[uuid.UUID]command.Cmd[any]),
		dispatched:     make(map[uuid.UUID]dispatcher),
		assigned:       make(map[uuid.UUID]dispatcher),
		pending:        make(map[uuid.UUID]dispatcher),
		assignTimeout:  DefaultAssignTimeout,
		receiveTimeout: DefaultReceiveTimeout,
		enc:            enc,
		bus:            events,
		handlerID:      uuid.New(),
	}
	for _, opt := range opts {
		opt(b)
	}

	event.HandleWith(b, b.commandDispatched, CommandDispatched)
	event.HandleWith(b, b.commandRequested, CommandRequested)
	event.HandleWith(b, b.commandAssigned, CommandAssigned)
	event.HandleWith(b, b.commandAccepted, CommandAccepted)
	event.HandleWith(b, b.commandExecuted, CommandExecuted)

	return b
}

// Run runs the command bus until ctx is canceled. If the bus is used before Run
// has been called, Run will be called automtically and the errors are logged to
// stderr.
func (b *Bus) Run(ctx context.Context) (<-chan error, error) {
	errs, err := b.Handler.Run(ctx)
	if err != nil {
		return errs, err
	}

	b.errs, b.fail = concurrent.Errors(ctx)

	return streams.FanInContext(ctx, b.errs, errs), nil
}

// Dispatch dispatches a Command to the appropriate handler (Command Bus) using
// the underlying Event Bus to communicate between b and the other Command Buses.
//
// How it works
//
// Dispatch first publishes a CommandDispatched Event with the Command Payload
// encoded in the Event Data. Every Command Bus that is currently subscribed to
// a Command receives the CommandDispatched Event and checks if it handles
// Commands that have the name of the dispatched Command.
//
// If a Command Bus doesn't handle Commands with that name, they just ignore the
// CommandDispatched Event, but if they're instructed to handle such Commands,
// they tell the Bus b that they want to handle the Command by publishing a
// CommandRequested Event which the Bus b will listen for.
//
// The first of those CommandRequested Events that the Bus b receives is used to
// assign the Command to a Handler. When b receives the first CommandRequested
// Event, it publishes a CommandAssigned Event with the ID of the selected
// Handler.
//
// The handler Command Buses receive the CommandAssigned Event and check if
// they're Handler that is assigned to the Command. The assigned Handler then
// publishes a final CommandAccepted Event to tell the Bus b that the Command
// arrived at its Handler.
//
// Errors
//
// By default, the error returned by Dispatch doesn't give any information about
// the execution of the Command because the Bus returns as soon as another Bus
// accepts a dispatched Command.
//
// To handle errors that happen during the execution of Commands, use the
// dispatch.Sync() Option to make the dispatch synchronous. A synchronous
// dispatch waits for and returns the execution error from the executing Bus.
//
// Errors that happen during a synchronous excecution are then also returned by
// Dispatch as an *ExecutionError. Call ExecError with that error as the
// argument to unwrap the underlying *ExecutionError:
//
//	var b command.Bus
//	err := b.Dispatch(context.TODO(), command.New(...))
//	if execError, ok := cmdbus.ExecError(err); ok {
//		log.Println(execError.Cmd)
//		log.Println(execError.Err)
//	}
//
// Execution result
//
// By default, Dispatch does not return information about the execution of a
// Command, but a report.Reporter can be provided with the dispatch.Report()
// Option. When a Reporter is provided, the dispatch is automatically made
// synchronous.
//
// Example:
//	var rep report.Report
//	var cmd command.Command
//	err := b.Dispatch(context.TODO(), cmd, dispatch.Report(&rep))
// 	log.Println(fmt.Sprintf("Command: %v", rep.Command()))
//	log.Println(fmt.Sprintf("Runtime: %v", rep.Runtime()))
// 	log.Println(fmt.Sprintf("Error: %v", err))
func (b *Bus) Dispatch(ctx context.Context, cmd command.Command, opts ...command.DispatchOption) (err error) {
	if !b.Running() {
		errs, err := b.Run(context.Background())
		if err != nil {
			return err
		}

		go logErrors(errs)
	}

	defer func() {
		if err != nil && errors.Is(err, context.Canceled) {
			err = fmt.Errorf("%w: %v", ErrDispatchCanceled, err)
		}
	}()

	cfg := dispatch.Configure(opts...)

	var load bytes.Buffer
	if err := b.enc.Encode(&load, cmd.Name(), cmd.Payload()); err != nil {
		return fmt.Errorf("encode payload: %w", err)
	}

	id, name := cmd.Aggregate()

	evt := event.New(CommandDispatched, CommandDispatchedData{
		ID:            cmd.ID(),
		Name:          cmd.Name(),
		AggregateName: name,
		AggregateID:   id,
		Payload:       load.Bytes(),
	})

	if err := b.bus.Publish(ctx, evt.Any()); err != nil {
		return fmt.Errorf("publish %q event: %w", evt.Name(), err)
	}

	out := make(chan error)
	accepted := make(chan struct{})

	b.mux.Lock()
	b.dispatched[cmd.ID()] = dispatcher{
		cmd:      cmd,
		cfg:      cfg,
		accepted: accepted,
		out:      out,
	}
	b.mux.Unlock()

	var timeout <-chan time.Time
	if b.assignTimeout > 0 {
		timer := time.NewTimer(b.assignTimeout)
		defer timer.Stop()
		timeout = timer.C
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timeout:
		return ErrAssignTimeout
	case <-accepted:
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err, failed := <-out:
		if failed {
			return err
		}
	}

	return nil
}

// Subscribe returns a channel of Command Contexts and an error channel. The
// Context channel channel is registered as a handler for Commands which have
// one of the specified names.
//
// Callers of Subscribe are responsible for receiving from the returned error
// channel to prevent a deadlock.
//
// When a Command Bus, which uses the same underlying Event Bus as Bus b,
// dispatches a Command, Bus b tries to assign itself as the handler for that
// Command. If b is assigned as the handler, a Command Context can be received
// from the returned channel.
//
// It is guaranteed that only one Command Bus will handle a single Command; when
// a Command is received from the Context channel, no other Context channel will
// receive that Command.
//
// When ctx is canceled, the remaining Commands that have already been received
// are pushed into the Context channel before it is closed. Use the DrainTimeout
// Option to specify the timeout after which the remaining Commands are being
// discarded.
func (b *Bus) Subscribe(ctx context.Context, names ...string) (<-chan command.Ctx[any], <-chan error, error) {
	if !b.Running() {
		errs, err := b.Run(context.Background())
		if err != nil {
			return nil, nil, err
		}

		go logErrors(errs)
	}

	out, errs := make(chan command.Context), make(chan error)

	if len(names) == 0 {
		return out, errs, nil
	}

	b.mux.Lock()
	defer b.mux.Unlock()

	for _, name := range names {
		if _, ok := b.subscriptions[name]; ok {
			return nil, nil, fmt.Errorf("%w: %s", ErrSubscribed, name)
		}
	}

	for _, name := range names {
		sub := &subscription{
			commands: out,
			errs:     errs,
		}
		b.subscriptions[name] = sub
	}

	// unsubscribe when the context is canceled
	go func() {
		<-ctx.Done()
		b.mux.Lock()
		defer b.mux.Unlock()

		for _, name := range names {
			if sub, ok := b.subscriptions[name]; ok {
				close(sub.commands)
				close(sub.errs)
			}
			delete(b.subscriptions, name)
		}
	}()

	return out, errs, nil
}

func (b *Bus) commandDispatched(evt event.Of[CommandDispatchedData]) {
	data := evt.Data()

	// if the bus does not handle the dispatched command, return
	if !b.handles(data.Name) {
		return
	}

	// otherwise request to become the handler of the command
	requestEvent := event.New(CommandRequested, CommandRequestedData{
		ID:        data.ID,
		HandlerID: b.handlerID,
	})

	if err := b.bus.Publish(b.Context(), requestEvent.Any()); err != nil {
		b.fail(fmt.Errorf("[goes/command/cmdbus.Bus@commandDispatched] Failed to request %q command: %w", data.Name, err))
		return
	}

	load, err := b.enc.Decode(bytes.NewReader(data.Payload), data.Name)
	if err != nil {
		b.fail(fmt.Errorf("[goes/command/cmdbus.Bus@commandDispatched] Failed to decode %q command: %w", data.Name, err))
		return
	}

	b.requested[data.ID] = command.New(data.Name, load, command.ID(data.ID), command.Aggregate(data.AggregateName, data.AggregateID))
}

func (b *Bus) handles(name string) bool {
	b.mux.RLock()
	defer b.mux.RUnlock()
	_, ok := b.subscriptions[name]
	return ok
}

func (b *Bus) commandRequested(evt event.Of[CommandRequestedData]) {
	data := evt.Data()

	// if the bus did not dispatch the command, return
	b.mux.RLock()
	cmd, ok := b.dispatched[data.ID]
	b.mux.RUnlock()
	if !ok {
		return
	}

	// otherwise remove the command from the dispatched commands
	delete(b.dispatched, data.ID)

	// and assign the command to the handler that requested to handle it
	assignEvent := event.New(CommandAssigned, CommandAssignedData{
		ID:        data.ID,
		HandlerID: data.HandlerID,
	})

	if err := b.bus.Publish(b.Context(), assignEvent.Any()); err != nil {
		b.fail(fmt.Errorf("[goes/command/cmdbus.Bus@commandRequested] Failed to assign %q command to handler %q: %w", cmd.cmd.Name(), data.HandlerID, err))
		return
	}

	b.assigned[data.ID] = cmd
}

func (b *Bus) commandAssigned(evt event.Of[CommandAssignedData]) {
	data := evt.Data()

	cmd := b.requested[data.ID]

	// otherwise remove the command from the requested commands
	delete(b.requested, data.ID)

	// and accept the command
	acceptEvt := event.New(CommandAccepted, CommandAcceptedData{
		ID:        data.ID,
		HandlerID: data.HandlerID,
	})

	if err := b.bus.Publish(b.Context(), acceptEvt.Any()); err != nil {
		b.fail(fmt.Errorf("[goes/command/cmdbus.Bus@commandAssigned] Failed to accept %q command: %w", cmd.Name(), err))
		return
	}

	// then pass the command to the subscription
	b.mux.RLock()
	defer b.mux.RUnlock()
	sub, ok := b.subscriptions[cmd.Name()]
	if !ok {
		return
	}

	var timeout <-chan time.Time
	if b.receiveTimeout > 0 {
		timer := time.NewTimer(b.receiveTimeout)
		defer timer.Stop()
		timeout = timer.C
	}

	select {
	case <-b.Context().Done():
		return
	case <-timeout:
		select {
		case <-b.Context().Done():
		case sub.errs <- fmt.Errorf("dropping %q command: %w", cmd.Name(), ErrReceiveTimeout):
		}
	case sub.commands <- command.NewContext[any](
		b.Context(),
		cmd,
		command.WhenDone(func(ctx context.Context, cfg finish.Config) error {
			return b.markDone(ctx, cmd, cfg)
		}),
	):
	}
}

func (b *Bus) markDone(ctx context.Context, cmd command.Command, cfg finish.Config) error {
	var errmsg string

	if cfg.Err != nil {
		errmsg = cfg.Err.Error()
	}

	evt := event.New(CommandExecuted, CommandExecutedData{
		ID:      cmd.ID(),
		Runtime: cfg.Runtime,
		Error:   errmsg,
	})

	if err := b.bus.Publish(ctx, evt.Any()); err != nil {
		return fmt.Errorf("publish %q event: %w", evt.Name(), err)
	}

	return nil
}

func (b *Bus) commandAccepted(evt event.Of[CommandAcceptedData]) {
	data := evt.Data()

	// if the bus did not assign the command, return
	cmd, ok := b.assigned[data.ID]
	if !ok {
		return
	}

	// otherwise mark the command as accepted and remove it from the assigned commands
	close(cmd.accepted)
	delete(b.assigned, data.ID)

	// if the dispatch was made synchronously, add the command to pending
	// commands, return and let b.commandExecuted do the rest
	if cmd.cfg.Synchronous || cmd.cfg.Reporter != nil {
		b.pending[data.ID] = cmd
		return
	}

	// otherwise close the error channel of the dispatcher
	close(cmd.out)
}

func (b *Bus) commandExecuted(evt event.Of[CommandExecutedData]) {
	data := evt.Data()

	// if the bus is not waiting for the execution of the command, return
	cmd, ok := b.pending[data.ID]
	if !ok {
		if cmd, ok = b.assigned[data.ID]; !ok {
			return
		}
	}

	// otherwise remove the command from pending and assigned commands
	delete(b.pending, data.ID)
	delete(b.assigned, data.ID)

	// if the dispatch requested a report, report the execution result
	if cmd.cfg.Reporter != nil {
		id, name := cmd.cmd.Aggregate()

		var err error
		if data.Error != "" {
			err = errors.New(data.Error)
		}

		cmd.cfg.Reporter.Report(report.New(report.Command{
			ID:            cmd.cmd.ID(),
			Name:          cmd.cmd.Name(),
			Payload:       cmd.cmd.Payload(),
			AggregateName: name,
			AggregateID:   id,
		}, report.Runtime(data.Runtime), report.Error(&ExecutionError[any]{
			Cmd: cmd.cmd,
			Err: err,
		})))
	}

	// if command execution failed, send the error to the dispatcher error channel and return
	if data.Error != "" {
		select {
		case <-b.Context().Done():
			return
		case cmd.out <- &ExecutionError[any]{
			Cmd: cmd.cmd,
			Err: errors.New(data.Error),
		}:
		}
		return
	}

	// otherwise close the error channel of the dispatcher
	close(cmd.out)
}

// logging errors to stderr if the command bus was started by Dispatch() or Subscribe().
func logErrors(errs <-chan error) {
	for err := range errs {
		log.Printf("[goes/command/cmdbus.logErrors] %v", err)
	}
}
