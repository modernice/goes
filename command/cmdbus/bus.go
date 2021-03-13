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
	// ErrAssignTimeout is returned by a Bus when it fails to assign a Command
	// to a Handler before a given deadline.
	ErrAssignTimeout = errors.New("failed to assign command because of timeout")

	// ErrDispatchCanceled is returned by a Bus when the dispatch was canceled
	// by the provided Context.
	ErrDispatchCanceled = errors.New("dispatch canceled")
)

// Bus is an Event-driven Command Bus.
type Bus struct {
	assignTimeout time.Duration
	drainTimeout  time.Duration

	enc command.Encoder
	bus event.Bus

	handlerID uuid.UUID
}

// Option is a Command Bus option.
type Option func(*Bus)

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
// remaining Commands after the Context that's used to subscribe to Commands is
// canceled.
//
// A zero Duration means no timeout. The default timeout is 10s.
func DrainTimeout(dur time.Duration) Option {
	return func(b *Bus) {
		b.drainTimeout = dur
	}
}

// New returns an event-driven Command Bus.
func New(enc command.Encoder, events event.Bus, opts ...Option) *Bus {
	b := Bus{
		assignTimeout: DefaultAssignTimeout,
		drainTimeout:  DefaultDrainTimeout,
		enc:           enc,
		bus:           events,
		handlerID:     uuid.New(),
	}
	for _, opt := range opts {
		opt(&b)
	}
	return &b
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
// To handle errors that happen during the execution of Commands, call
// b.Errors() on the receiving Command Bus instead.
//
// Alternatively, use the dispatch.Synchronous() Option to make the dispatch
// synchronous. A synchronous dispatch waits for the handling Bus to publish a
// CommandExecuted Event before returning.
//
// Errors that happen during the excecution of the Command are then also
// returned by Dispatch as an *ExecutionError. Call ExecError with that error as
// the argument to unwrap the underlying *ExecutionError:
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
func (b *Bus) Dispatch(ctx context.Context, cmd command.Command, opts ...command.DispatchOption) error {
	cfg := dispatch.Configure(opts...)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	events, errs, err := b.subscribeDispatch(ctx, cfg.Synchronous)
	if err != nil {
		return b.dispatchError(err)
	}

	if err := b.dispatch(ctx, cmd); err != nil {
		return b.dispatchError(err)
	}

	var assignTimeout <-chan time.Time
	if b.assignTimeout > 0 {
		timer := time.NewTimer(b.assignTimeout)
		defer timer.Stop()
		assignTimeout = timer.C
	}

	return b.dispatchError(b.workDispatch(
		ctx,
		cfg,
		cmd,
		events,
		errs,
		assignTimeout,
	))
}

func (b *Bus) dispatchError(err error) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, context.Canceled) {
		return ErrDispatchCanceled
	}

	return err
}

func (b *Bus) subscribeDispatch(ctx context.Context, sync bool) (<-chan event.Event, <-chan error, error) {
	names := []string{CommandRequested, CommandAccepted}
	if sync {
		names = append(names, CommandExecuted)
	}

	events, errs, err := b.bus.Subscribe(ctx, names...)
	if err != nil {
		return nil, nil, fmt.Errorf("subscribe to %v events: %w", names, err)
	}

	return events, errs, nil
}

func (b *Bus) dispatch(ctx context.Context, cmd command.Command) error {
	var load bytes.Buffer
	if err := b.enc.Encode(&load, cmd.Name(), cmd.Payload()); err != nil {
		return fmt.Errorf("encode payload: %w", err)
	}

	evt := event.New(CommandDispatched, CommandDispatchedData{
		ID:            cmd.ID(),
		Name:          cmd.Name(),
		AggregateName: cmd.AggregateName(),
		AggregateID:   cmd.AggregateID(),
		Payload:       load.Bytes(),
	})

	if err := b.bus.Publish(ctx, evt); err != nil {
		return fmt.Errorf("publish %q event: %w", evt.Name(), err)
	}

	return nil
}

func (b *Bus) workDispatch(
	ctx context.Context,
	cfg command.DispatchConfig,
	cmd command.Command,
	events <-chan event.Event,
	errs <-chan error,
	assignTimeout <-chan time.Time,
) error {
	accepted := make(chan struct{})

	for {
		select {
		case <-assignTimeout:
			return ErrAssignTimeout
		case err, ok := <-errs:
			if ok {
				return fmt.Errorf("event stream: %w", err)
			}
			errs = nil
		case evt, ok := <-events:
			if !ok {
				return ErrDispatchCanceled
			}

			if done, err := b.handleDispatchEvent(ctx, cfg, cmd, evt, accepted); done {
				return err
			}

			select {
			case <-accepted:
				assignTimeout = nil
			default:
			}
		}
	}
}

func (b *Bus) handleDispatchEvent(
	ctx context.Context,
	cfg command.DispatchConfig,
	cmd command.Command,
	evt event.Event,
	accepted chan struct{},
) (bool, error) {
	switch evt.Name() {
	case CommandRequested:
		data := evt.Data().(CommandRequestedData)
		if data.ID != cmd.ID() {
			return false, nil
		}

		if err := b.assignCommand(ctx, cmd, data); err != nil {
			return true, fmt.Errorf("assign command: %w", err)
		}

		return false, nil

	case CommandAccepted:
		data := evt.Data().(CommandAcceptedData)
		if data.ID != cmd.ID() {
			return false, nil
		}
		if !cfg.Synchronous {
			return true, nil
		}
		close(accepted)
		return false, nil

	case CommandExecuted:
		data := evt.Data().(CommandExecutedData)
		if data.ID != cmd.ID() {
			return false, nil
		}

		var err error
		if data.Error != "" {
			err = &ExecutionError{
				Cmd: cmd,
				Err: errors.New(data.Error),
			}
		}

		if cfg.Reporter != nil {
			cfg.Reporter.Report(report.New(
				cmd,
				report.Error(err),
				report.Runtime(data.Runtime),
			))
		}

		return true, err
	default:
		return false, nil
	}
}

func (b *Bus) assignCommand(ctx context.Context, cmd command.Command, data CommandRequestedData) error {
	if err := b.bus.Publish(ctx, event.New(CommandAssigned, CommandAssignedData{
		ID:        data.ID,
		HandlerID: b.handlerID,
	})); err != nil {
		return fmt.Errorf("publish %q event: %w", CommandAssigned, err)
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
func (b *Bus) Subscribe(ctx context.Context, names ...string) (<-chan command.Context, <-chan error, error) {
	events, errs, err := b.subscribeSubscribe(ctx)
	if err != nil {
		return nil, nil, err
	}

	out, outErrs := make(chan command.Context), make(chan error)

	go b.workSubscription(ctx, events, errs, out, outErrs)

	return out, outErrs, nil
}

func (b *Bus) subscribeSubscribe(ctx context.Context) (<-chan event.Event, <-chan error, error) {
	names := []string{CommandDispatched, CommandAssigned}
	events, errs, err := b.bus.Subscribe(ctx, names...)
	if err != nil {
		return nil, nil, fmt.Errorf("subscribe to %v events: %w", names, err)
	}
	return events, errs, nil
}

func (b *Bus) workSubscription(
	parentCtx context.Context,
	events <-chan event.Event,
	errs <-chan error,
	out chan<- command.Context,
	outErrs chan<- error,
) {
	type commandRequest struct {
		cmd  command.Command
		time time.Time
	}

	isDone := make(chan struct{})
	defer close(isDone)

	defer close(out)
	defer close(outErrs)

	ctx := b.newSubscriptionContext(parentCtx, isDone)

	requested := make(map[uuid.UUID]commandRequest)

	for {
		select {
		case err, ok := <-errs:
			if !ok {
				errs = nil
				break
			}
			outErrs <- fmt.Errorf("event stream: %w", err)
		case evt, ok := <-events:
			if !ok {
				return
			}

			switch evt.Name() {
			case CommandDispatched:
				data := evt.Data().(CommandDispatchedData)
				load, err := b.enc.Decode(data.Name, bytes.NewReader(data.Payload))
				if err != nil {
					outErrs <- fmt.Errorf("decode payload: %w", err)
					break
				}

				cmd := command.New(
					data.Name,
					load,
					command.ID(data.ID),
					command.Aggregate(data.AggregateName, data.AggregateID),
				)

				if err := b.requestCommand(ctx, cmd); err != nil {
					outErrs <- fmt.Errorf("request command: %w", err)
					break
				}

				requested[cmd.ID()] = commandRequest{
					cmd:  cmd,
					time: time.Now(),
				}

			case CommandAssigned:
				data := evt.Data().(CommandAssignedData)
				if data.HandlerID != b.handlerID {
					delete(requested, data.ID)
					break
				}

				req, ok := requested[data.ID]
				if !ok {
					break
				}
				delete(requested, data.ID)

				if err := b.acceptCommand(ctx, req.cmd); err != nil {
					outErrs <- fmt.Errorf("accept command: %w", err)
					break
				}

				select {
				case <-ctx.Done():
					return
				case out <- cmdctx.New(
					context.Background(),
					req.cmd,
					cmdctx.WhenDone(func(ctx context.Context, cfg done.Config) error {
						return b.markDone(ctx, req.cmd, cfg)
					}),
				):
				}
			}
		}
	}
}

func (b *Bus) newSubscriptionContext(parent context.Context, done <-chan struct{}) context.Context {
	if b.drainTimeout == 0 {
		return context.Background()
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		defer cancel()
		select {
		case <-done:
			return
		case <-parent.Done():
		}

		timer := time.NewTimer(b.drainTimeout)
		defer timer.Stop()

		select {
		case <-done:
		case <-timer.C:
		}
	}()

	return ctx
}

func (b *Bus) requestCommand(ctx context.Context, cmd command.Command) error {
	evt := event.New(CommandRequested, CommandRequestedData{
		ID:        cmd.ID(),
		HandlerID: b.handlerID,
	})
	if err := b.bus.Publish(ctx, evt); err != nil {
		return fmt.Errorf("publish %q event: %w", evt.Name(), err)
	}
	return nil
}

func (b *Bus) acceptCommand(ctx context.Context, cmd command.Command) error {
	evt := event.New(CommandAccepted, CommandAcceptedData{
		ID:        cmd.ID(),
		HandlerID: b.handlerID,
	})
	if err := b.bus.Publish(ctx, evt); err != nil {
		return fmt.Errorf("publish %q event: %w", evt.Name(), err)
	}
	return nil
}

func (b *Bus) markDone(ctx context.Context, cmd command.Command, cfg done.Config) error {
	var errmsg string
	if cfg.Err != nil {
		errmsg = cfg.Err.Error()
	}
	evt := event.New(CommandExecuted, CommandExecutedData{
		ID:      cmd.ID(),
		Runtime: cfg.Runtime,
		Error:   errmsg,
	})
	if err := b.bus.Publish(ctx, evt); err != nil {
		return fmt.Errorf("publish %q event: %w", evt.Name(), err)
	}
	return nil
}
