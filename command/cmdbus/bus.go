// Package cmdbus provides a distributed & event-driven Command Bus.
package cmdbus

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/command"
	"github.com/modernice/goes/command/cmdbus/dispatch"
	"github.com/modernice/goes/command/cmdbus/report"
	"github.com/modernice/goes/command/done"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/internal/errbus"
	"github.com/modernice/goes/internal/xcommand/cmdctx"
)

const (
	// DefaultAssignTimeout is the default timeout when assigning a Command to a
	// Handler.
	DefaultAssignTimeout = 5 * time.Second

	// DefaultDrainTimeout is the default timeout for accepting Commands after
	// the used context is canceled.
	DefaultDrainTimeout = 10 * time.Second
)

var (
	// ErrAssignTimeout is returned by the Bus when it fails to assign a Command
	// to a Handler before a specified deadline.
	ErrAssignTimeout = errors.New("failed to assign command because of timeout")
)

// Bus is an Event-driven Command Bus.
type Bus struct {
	assignTimeout time.Duration
	drainTimeout  time.Duration

	enc  command.Encoder
	bus  event.Bus
	errs *errbus.Bus
}

// Option is a Command Bus option.
type Option func(*Bus)

type pendingCommand struct {
	Cmd       command.Command
	HandlerID uuid.UUID
}

// AssignTimeout returns an Option that configures the timeout when assigning a
// Command to a Handler. A zero Duration means no timeout.
//
// A zero Duration means no timeout. The default timeout is 5s.
func AssignTimeout(dur time.Duration) Option {
	return func(b *Bus) {
		b.assignTimeout = dur
	}
}

// DrainTimeout returns an Option that configures the timeout when accepting the
// remaining Commands after the Context that's used to subscribe to Command is
// canceled.
//
// A zero Duration means no timeout. The default timeout is 10s.
func DrainTimeout(dur time.Duration) Option {
	return func(b *Bus) {
		b.drainTimeout = dur
	}
}

// New returns an Event-driven Command Bus.
func New(enc command.Encoder, events event.Bus, opts ...Option) *Bus {
	b := Bus{
		assignTimeout: DefaultAssignTimeout,
		drainTimeout:  DefaultDrainTimeout,
		enc:           enc,
		bus:           events,
		errs:          errbus.New(),
	}
	for _, opt := range opts {
		opt(&b)
	}
	return &b
}

// Dispatch dispatches a Command to the appropriate Handler (Command Bus) using
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
// To handle errors that happen during the execution of Commands, call
// b.Errors() on the receiving Command Bus instead.
//
// Alternatively, use the dispatch.Synchronous() Option to make the dispatch
// synchronous. A synchronous dispatch waits for the handling Bus to publish a
// CommandExecuted Event before returning.
//
// Errors that happen during the excecution of the Command are then also
// returned by Dispatch as an *ExecutionError. Call ExecError() with
// that error as the argument to unwrap the underlying *ExecutionError:
//	err := b.Dispatch(context.TODO(), cmd)
//	if execError, ok := ExecError(err); ok {
//		log.Println(execError.Cmd)
//		log.Println(execError.Err)
//	}
//
// Execution result
//
// By default, Dispatch does not return information about the execution of a
// Command, but a report.Reporter can be provided with the dispatch.Report()
// Option. When a Reporter is provided, the dispatch is automatically made
// synchronous because the Bus has to wait until Command execution completes.
//
// Example:
//	var rep report.Report
//	err := b.Dispatch(context.TODO(), cmd, dispatch.Report(&rep))
// 	// handle err
// 	log.Println(fmt.Sprintf("Command: %v", rep.Command()))
//	log.Println(fmt.Sprintf("Runtime: %v", rep.Runtime()))
// 	log.Println(fmt.Sprintf("Error: %v", rep.Error()))
func (b *Bus) Dispatch(ctx context.Context, cmd command.Command, opts ...dispatch.Option) error {
	cfg := dispatch.Configure(opts...)
	sync := cfg.Synchronous
	if cfg.Reporter != nil {
		sync = true
	}

	var load bytes.Buffer
	if err := b.enc.Encode(&load, cmd.Payload()); err != nil {
		return fmt.Errorf("encode payload: %w", err)
	}

	// assign command to handler
	assignedc, errc, err := b.assignHandler(ctx, cmd.ID())
	if err != nil {
		return fmt.Errorf("assign handler: %w", err)
	}

	// if the dispatch is synchronous, retrieve the execution error
	var resultc <-chan CommandExecutedData
	if sync {
		if resultc, err = b.awaitResult(ctx, cmd.ID()); err != nil {
			return fmt.Errorf("failed to await result: %w", err)
		}
	}

	// publish `CommandDispatched` event
	if err := b.bus.Publish(ctx, event.New(CommandDispatched, CommandDispatchedData{
		ID:            cmd.ID(),
		Name:          cmd.Name(),
		AggregateName: cmd.AggregateName(),
		AggregateID:   cmd.AggregateID(),
		Payload:       load.Bytes(),
	})); err != nil {
		return fmt.Errorf("publish %q event: %w", CommandDispatched, err)
	}

	// wait until the command has been assigned to a handler
	if err := b.awaitAssigned(ctx, assignedc, errc); err != nil {
		return err
	}

	// if the dispatch is synchronous, await the execution result
	if sync {
		return b.executionResult(ctx, cfg, cmd, resultc)
	}

	return nil
}

func (b *Bus) awaitAssigned(ctx context.Context, assigned <-chan struct{}, errc <-chan error) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errc:
		if errors.Is(err, context.DeadlineExceeded) {
			err = ErrAssignTimeout
		}
		return fmt.Errorf("assign handler: %w", err)
	case <-assigned:
		return nil
	}
}

func (b *Bus) executionResult(
	ctx context.Context,
	cfg dispatch.Config,
	cmd command.Command,
	result <-chan CommandExecutedData,
) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case data, ok := <-result:
		if !ok {
			return nil
		}

		var execError *ExecutionError
		if data.Error != "" {
			execError = &ExecutionError{
				Cmd: cmd,
				Err: errors.New(data.Error),
			}
		}

		if cfg.Reporter != nil {
			opts := []report.Option{report.Runtime(data.Runtime)}
			if execError != nil {
				opts = append(opts, report.Error(execError))
			}
			cfg.Reporter.Report(cmd, opts...)
		}

		if execError != nil {
			return execError
		}
	}
	return nil
}

func (b *Bus) assignHandler(
	ctx context.Context,
	cmdID uuid.UUID,
) (<-chan struct{}, <-chan error, error) {
	assigned := make(chan struct{})
	errc := make(chan error)

	var cancel context.CancelFunc
	if b.assignTimeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, b.assignTimeout)
	} else {
		ctx, cancel = context.WithCancel(ctx)
	}

	requested, err := b.bus.Subscribe(ctx, CommandRequested)
	if err != nil {
		cancel()
		return nil, nil, fmt.Errorf(
			"subscribe to %q events: %w",
			CommandRequested,
			err,
		)
	}

	go func() {
		defer cancel()
		for {
			var (
				evt event.Event
				ok  bool
			)

			select {
			case <-ctx.Done():
				errc <- ctx.Err()
				return
			case evt, ok = <-requested:
				if !ok {
					errc <- ErrAssignTimeout
					return
				}
			}

			data := evt.Data().(CommandRequestedData)
			if data.ID != cmdID {
				continue
			}

			if err := b.assignHandlerTo(
				ctx,
				cmdID,
				data.HandlerID,
			); err != nil {
				errc <- fmt.Errorf("assign handler: %w", err)
				return
			}
			close(assigned)
			return
		}
	}()

	return assigned, errc, nil
}

func (b *Bus) assignHandlerTo(ctx context.Context, cmdID, handlerID uuid.UUID) error {
	evt := event.New(CommandAssigned, CommandAssignedData{
		ID:        cmdID,
		HandlerID: handlerID,
	})
	if err := b.bus.Publish(ctx, evt); err != nil {
		return fmt.Errorf("publish %q events: %w", CommandAssigned, err)
	}
	return nil
}

func (b *Bus) awaitResult(ctx context.Context, cmdID uuid.UUID) (<-chan CommandExecutedData, error) {
	ctx, cancel := context.WithCancel(ctx)

	events, err := b.bus.Subscribe(ctx, CommandExecuted)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("subscribe to %q events: %w", CommandExecuted, err)
	}

	result := make(chan CommandExecutedData, 1)
	go func() {
		defer cancel()
		defer close(result)
		for evt := range events {
			data := evt.Data().(CommandExecutedData)
			if data.ID != cmdID {
				continue
			}

			select {
			case <-ctx.Done():
				return
			case result <- data:
				return
			}
		}
	}()

	return result, nil
}

// Subscribe returns a channel of Command Contexts. That channel is registered as
// a handler for Commands which have one of the specified names.
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
func (b *Bus) Subscribe(ctx context.Context, names ...string) (<-chan command.Context, error) {
	dispatched, err := b.bus.Subscribe(ctx, CommandDispatched)
	if err != nil {
		return nil, fmt.Errorf(
			"subscribe to %q events: %w",
			CommandDispatched,
			err,
		)
	}

	commands := make(chan pendingCommand)
	requested := make(chan pendingCommand)
	assigned := make(chan pendingCommand)
	accepted := make(chan command.Context, 1)

	// handle `CommandDispatched` events
	go b.handleDispatched(ctx, dispatched, commands)

	// request dispatched commands
	go b.requestCommands(ctx, commands, requested)

	// handle assigned commands
	go b.handleAssign(ctx, requested, assigned)

	// use a separate context for accepting commands because assigned commands
	// should be handled even after ctx is canceled
	acceptCtx, cancel := context.WithCancel(context.Background())

	// done is closed by b.acceptCommands after all commands are accepted
	done := make(chan struct{})

	// stop accepting commands when ctx or done is closed
	go b.stopAccept(ctx, done, cancel)

	// accept assigned commands
	go b.acceptCommands(acceptCtx, assigned, accepted, done)

	return accepted, nil
}

func (b *Bus) handleDispatched(
	ctx context.Context,
	events <-chan event.Event,
	request chan<- pendingCommand,
) {
	defer close(request)
	for evt := range events {
		b.handleDispatchedEvent(ctx, evt, request)
	}
}

func (b *Bus) handleDispatchedEvent(
	ctx context.Context,
	evt event.Event,
	request chan<- pendingCommand,
) {
	data, ok := evt.Data().(CommandDispatchedData)
	if !ok {
		b.error(fmt.Errorf(
			"invalid event data type. want=%T got=%T",
			CommandDispatchedData{},
			evt.Data(),
		))
		return
	}

	load, err := b.enc.Decode(data.Name, bytes.NewReader(data.Payload))
	if err != nil {
		b.error(fmt.Errorf("decode %q command payload: %w", data.Name, err))
		return
	}

	cmd := command.New(
		data.Name,
		load,
		command.ID(data.ID),
		command.Aggregate(data.AggregateName, data.AggregateID),
	)

	select {
	case <-ctx.Done():
	case request <- pendingCommand{Cmd: cmd}:
	}
}

func (b *Bus) requestCommands(
	ctx context.Context,
	cmds <-chan pendingCommand,
	requested chan<- pendingCommand,
) {
	defer close(requested)
	for cmd := range cmds {
		if err := b.requestCommand(ctx, cmd, requested); err != nil {
			b.error(fmt.Errorf("accept command: %w", err))
		}
	}
}

func (b *Bus) requestCommand(
	ctx context.Context,
	cmd pendingCommand,
	requested chan<- pendingCommand,
) error {
	handlerID := uuid.New()
	evt := event.New(CommandRequested, CommandRequestedData{
		ID:        cmd.Cmd.ID(),
		HandlerID: handlerID,
	})
	if err := b.bus.Publish(ctx, evt); err != nil {
		return fmt.Errorf("publish %q event: %w", CommandRequested, err)
	}

	cmd.HandlerID = handlerID
	select {
	case <-ctx.Done():
		return ctx.Err()
	case requested <- cmd:
		return nil
	}
}

func (b *Bus) handleAssign(
	ctx context.Context,
	accepted <-chan pendingCommand,
	assigned chan<- pendingCommand,
) {
	defer close(assigned)
	for pcmd := range accepted {
		if err := b.selfAssign(ctx, pcmd, assigned); err != nil {
			b.error(fmt.Errorf(
				"self-assign %q command: %w",
				pcmd.Cmd.Name(),
				err,
			))
		}
	}
}

func (b *Bus) selfAssign(
	ctx context.Context,
	pcmd pendingCommand,
	assigned chan<- pendingCommand,
) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	events, err := b.bus.Subscribe(ctx, CommandAssigned)
	if err != nil {
		return fmt.Errorf("subscribe to %q events: %w", CommandAssigned, err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case evt, ok := <-events:
			if !ok {
				return nil
			}
			data := evt.Data().(CommandAssignedData)
			if data.ID != pcmd.Cmd.ID() {
				continue
			}
			if data.HandlerID != pcmd.HandlerID {
				return nil
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			case assigned <- pcmd:
				return nil
			}
		}
	}
}

func (b *Bus) acceptCommands(
	ctx context.Context,
	assigned <-chan pendingCommand,
	accepted chan<- command.Context,
	done chan struct{},
) {
	defer close(done)
	defer close(accepted)
	for pcmd := range assigned {
		evt := event.New(CommandAccepted, CommandAcceptedData{
			ID:        pcmd.Cmd.ID(),
			HandlerID: pcmd.HandlerID,
		})

		if err := b.bus.Publish(ctx, evt); err != nil {
			b.error(fmt.Errorf("publish %q events: %w", evt.Name(), err))
			continue
		}

		select {
		case <-ctx.Done():
			b.error(fmt.Errorf(
				"handle accepted command (%s): %w",
				pcmd.Cmd.ID(),
				ctx.Err(),
			))
			return
		case accepted <- cmdctx.New(
			ctx,
			pcmd.Cmd,
			cmdctx.WhenDone(b.doneFunc(pcmd)),
		):
		}
	}
}

func (b *Bus) doneFunc(cmd pendingCommand) func(context.Context, ...done.Option) error {
	start := time.Now()
	return func(ctx context.Context, opts ...done.Option) error {
		return b.markDone(ctx, cmd, start, opts...)
	}
}

func (b *Bus) markDone(ctx context.Context, cmd pendingCommand, start time.Time, opts ...done.Option) error {
	cfg := done.Configure(opts...)
	var msg string
	if cfg.Err != nil {
		msg = cfg.Err.Error()
	}

	runtime := cfg.Runtime
	if runtime == 0 {
		runtime = time.Now().Sub(start)
	}

	if msg != "" {
		b.error(&ExecutionError{
			Cmd: cmd.Cmd,
			Err: errors.New(msg),
		})
	}

	evt := event.New(CommandExecuted, CommandExecutedData{
		ID:      cmd.Cmd.ID(),
		Runtime: runtime,
		Error:   msg,
	})
	if err := b.bus.Publish(ctx, evt); err != nil {
		return fmt.Errorf("publish %q event: %w", evt.Name(), err)
	}
	return nil
}

func (b *Bus) stopAccept(ctx context.Context, done chan struct{}, cancel context.CancelFunc) {
	defer cancel()
	select {
	// when done, just return
	case <-done:
		return
	// ctx canceled, now accept remaining commands
	case <-ctx.Done():
	}
	var timeout <-chan time.Time
	if b.drainTimeout > 0 {
		timer := time.NewTimer(b.drainTimeout)
		defer timer.Stop()
		timeout = timer.C
	}
	select {
	// wait until all commands are accepted
	case <-done:
	// or until the drain timeout is exceeded
	case <-timeout:
	}
}

// Errors returns a channel of asynchronous errors that happen while a Bus tries
// to communicate and coordinate with other Buses asynchronously over an Event
// Bus.
func (b *Bus) Errors(ctx context.Context) <-chan error {
	return b.errs.Subscribe(ctx)
}

func (b *Bus) error(err error) {
	b.errs.Publish(context.Background(), err)
}
