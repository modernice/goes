// Package cmdbus provides a distributed & event-driven Command Bus.
package cmdbus

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes"
	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/command"
	"github.com/modernice/goes/command/cmdbus/dispatch"
	"github.com/modernice/goes/command/cmdbus/report"
	"github.com/modernice/goes/command/finish"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/internal/xtime"
)

var _ command.Bus[uuid.UUID] = (*Bus[uuid.UUID])(nil)

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

	// ErrDrainTimeout is emitted by a Bus when the DrainTimeout is exceeded
	// when receiving remaining Commands from a canceled Command subscription.
	ErrDrainTimeout = errors.New("dropped command because of timeout")

	// ErrDispatchCanceled is returned by a Bus when the dispatch was canceled
	// by the provided Context.
	ErrDispatchCanceled = errors.New("dispatch canceled")
)

// Bus is an event-driven Command Bus.
type Bus[ID goes.ID] struct {
	options

	enc codec.Encoding
	bus event.Bus[ID]

	handlerID uuid.UUID

	logger *log.Logger
}

// Option is a Command Bus option.
type Option func(*options)

type options struct {
	newID         func() any
	assignTimeout time.Duration
	drainTimeout  time.Duration
	debug         bool
	debugID       string
}

// Debug enables verbose logging for debugging purposes. Optional id may be
// specified to annotate debug output.
func Debug(id string) Option {
	return func(opts *options) {
		opts.debug = true
		opts.debugID = id
	}
}

// AssignTimeout returns an Option that configures the timeout when assigning a
// Command to a Handler. A zero Duration means no timeout.
//
// A zero Duration means no timeout. The default timeout is 5s.
func AssignTimeout(dur time.Duration) Option {
	return func(opts *options) {
		opts.assignTimeout = dur
	}
}

// DrainTimeout returns an Option that configures the timeout when accepting the
// remaining Commands after the Context that's used to subscribe to Commands is
// canceled.
//
// A zero Duration means no timeout. The default timeout is 10s.
func DrainTimeout(dur time.Duration) Option {
	return func(opts *options) {
		opts.drainTimeout = dur
	}
}

// New returns an event-driven command bus.
func New[ID goes.ID](newID func() ID, enc codec.Encoding, events event.Bus[ID], opts ...Option) *Bus[ID] {
	options := options{
		newID:         func() any { return newID() },
		assignTimeout: DefaultAssignTimeout,
		drainTimeout:  DefaultDrainTimeout,
	}
	for _, opt := range opts {
		opt(&options)
	}

	b := Bus[ID]{
		options:   options,
		enc:       enc,
		bus:       events,
		handlerID: uuid.New(),
	}

	if b.debug {
		prefix := "[goes/command/cmdbus.Bus:"
		if b.debugID != "" {
			prefix += b.debugID + "] "
		} else {
			prefix += b.handlerID.String() + "] "
		}
		b.logger = log.New(os.Stderr, prefix, log.LstdFlags)
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
func (b *Bus[ID]) Dispatch(ctx context.Context, cmd command.Of[any, ID], opts ...command.DispatchOption) error {
	var err error
	b.debugMeasure(fmt.Sprintf("[dispatch] Dispatching %q command", cmd.Name()), func() {
		cfg := dispatch.Configure(opts...)

		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		var (
			events <-chan event.Of[any, ID]
			errs   <-chan error
		)

		events, errs, err = b.subscribeDispatch(ctx, cfg.Synchronous)
		if err != nil {
			err = b.dispatchError(err)
			return
		}

		if err = b.dispatch(ctx, cmd); err != nil {
			err = b.dispatchError(err)
			return
		}

		var assignTimeout <-chan time.Time
		if b.assignTimeout > 0 {
			timer := time.NewTimer(b.assignTimeout)
			defer timer.Stop()
			assignTimeout = timer.C
		}

		err = b.dispatchError(b.workDispatch(
			ctx,
			cfg,
			cmd,
			events,
			errs,
			assignTimeout,
		))
	})

	return err
}

func (b *Bus[ID]) dispatchError(err error) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, context.Canceled) {
		return ErrDispatchCanceled
	}

	return err
}

func (b *Bus[ID]) subscribeDispatch(ctx context.Context, sync bool) (<-chan event.Of[any, ID], <-chan error, error) {
	// ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	// defer cancel()

	names := []string{CommandRequested, CommandAccepted}
	if sync {
		names = append(names, CommandExecuted)
	}

	b.debugLog("[dispatch] Subscribing to %v events...", names)

	events, errs, err := b.bus.Subscribe(ctx, names...)
	if err != nil {
		return nil, nil, fmt.Errorf("subscribe to %v events: %w", names, err)
	}

	return events, errs, nil
}

func (b *Bus[ID]) dispatch(ctx context.Context, cmd command.Of[any, ID]) error {
	var load bytes.Buffer
	if err := b.enc.Encode(&load, cmd.Name(), cmd.Payload()); err != nil {
		return fmt.Errorf("encode payload: %w", err)
	}

	id, name := cmd.Aggregate()

	evt := event.New(b.newID().(ID), CommandDispatched, CommandDispatchedData[ID]{
		ID:            cmd.ID(),
		Name:          cmd.Name(),
		AggregateName: name,
		AggregateID:   id,
		Payload:       load.Bytes(),
	})

	b.debugLog("[dispatch] Publishing %q event...", evt.Name())

	var err error
	b.debugMeasure(fmt.Sprintf("[dispatch] Publishing %q event", evt.Name()), func() {
		// ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		// defer cancel()
		err = b.bus.Publish(ctx, evt.Any())
	})

	if err != nil {
		return fmt.Errorf("publish %q event: %w", evt.Name(), err)
	}

	return nil
}

func (b *Bus[ID]) workDispatch(
	ctx context.Context,
	cfg command.DispatchConfig,
	cmd command.Of[any, ID],
	events <-chan event.Of[any, ID],
	errs <-chan error,
	assignTimeout <-chan time.Time,
) error {
	var status dispatchStatus

	for {
		select {
		case <-ctx.Done():
			b.debugLog("[dispatch] Dispatch of %q command canceled because of canceled Context.")
			return ErrDispatchCanceled
		case <-assignTimeout:
			b.debugLog("[dispatch] Dispatch of %q command canceled because of AssignTimeout (%v).", cmd.Name(), b.assignTimeout)
			return fmt.Errorf("assign %q Command: %w", cmd.Name(), ErrAssignTimeout)
		case err, ok := <-errs:
			if ok {
				return fmt.Errorf("event stream: %w", err)
			}
			errs = nil
		case evt, ok := <-events:
			if !ok {
				events = nil
				break
			}

			var err error
			status, err = b.handleDispatchEvent(ctx, cfg, cmd, evt, status)

			if status.executed || (!cfg.Synchronous && status.accepted) {
				return err
			}

			if status.accepted {
				assignTimeout = nil
			}
		}
	}
}

type dispatchStatus struct {
	assigned bool
	accepted bool
	executed bool
}

func (b *Bus[ID]) handleDispatchEvent(
	ctx context.Context,
	cfg command.DispatchConfig,
	cmd command.Of[any, ID],
	evt event.Of[any, ID],
	status dispatchStatus,
) (dispatchStatus, error) {
	b.debugLog("[dispatch] Handling %q event (%s)...", evt.Name(), evt.ID())

	// log.Printf("EVENT %v", evt.Name())

	switch evt.Name() {
	case CommandRequested:
		data := evt.Data().(CommandRequestedData[ID])
		if data.ID != cmd.ID() {
			return status, nil
		}

		if err := b.assignCommand(ctx, cmd, data); err != nil {
			return status, fmt.Errorf("assign command: %w", err)
		}

		status.assigned = true
		b.debugLog("[dispatch] %q command assigned.", cmd.Name())

		return status, nil

	case CommandAccepted:
		data := evt.Data().(CommandAcceptedData[ID])
		if data.ID != cmd.ID() {
			return status, nil
		}
		status.accepted = true
		b.debugLog("[dispatch] %q command accepted.", cmd.Name())
		return status, nil

	case CommandExecuted:
		data := evt.Data().(CommandExecutedData[ID])
		if data.ID != cmd.ID() {
			return status, nil
		}

		var err error
		if data.Error != "" {
			err = &ExecutionError[any, ID]{
				Cmd: cmd,
				Err: errors.New(data.Error),
			}
		}

		if cfg.Reporter != nil {
			id, name := cmd.Aggregate()
			rep := report.New(
				report.Command[ID]{
					Name:          cmd.Name(),
					ID:            cmd.ID(),
					AggregateName: name,
					AggregateID:   id,
					Payload:       cmd.Payload(),
				},
				report.Error[ID](err),
				report.Runtime[ID](data.Runtime),
			)

			b.debugLog("[dispatch] Reporting %q command: %v", cmd.Name(), rep)

			if reporter, ok := cfg.Reporter.(command.Reporter[ID]); ok {
				reporter.Report(rep)
			}
		}

		status.executed = true
		return status, err
	default:
		return status, nil
	}
}

func (b *Bus[ID]) assignCommand(ctx context.Context, cmd command.Of[any, ID], data CommandRequestedData[ID]) error {
	evt := event.New(b.newID().(ID), CommandAssigned, CommandAssignedData[ID](data))

	var err error
	b.debugMeasure(fmt.Sprintf("[dispatch] Publishing %q event", evt.Name()), func() {
		err = b.bus.Publish(ctx, evt.Any())
	})
	if err != nil {
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
func (b *Bus[ID]) Subscribe(ctx context.Context, names ...string) (<-chan command.ContextOf[any, ID], <-chan error, error) {
	events, errs, err := b.subscribeSubscribe(ctx)
	if err != nil {
		return nil, nil, err
	}

	out, outErrs := make(chan command.ContextOf[any, ID]), make(chan error)

	go b.workSubscription(ctx, events, errs, out, outErrs, names)

	return out, outErrs, nil
}

func (b *Bus[ID]) subscribeSubscribe(ctx context.Context) (events <-chan event.Of[any, ID], errs <-chan error, err error) {
	names := []string{CommandDispatched, CommandAssigned}
	b.debugMeasure(fmt.Sprintf("[subscribe] Subscribing to %q events", names), func() {
		events, errs, err = b.bus.Subscribe(ctx, names...)
	})
	if err != nil {
		return nil, nil, fmt.Errorf("subscribe to %v events: %w", names, err)
	}
	return events, errs, nil
}

type commandRequest[ID goes.ID] struct {
	cmd  command.Of[any, ID]
	time time.Time
}

func (b *Bus[ID]) workSubscription(
	parentCtx context.Context,
	events <-chan event.Of[any, ID],
	errs <-chan error,
	out chan<- command.ContextOf[any, ID],
	outErrs chan<- error,
	names []string,
) {
	isDone := make(chan struct{})
	defer close(isDone)

	defer close(out)
	defer close(outErrs)

	ctx := b.newSubscriptionContext(parentCtx, isDone)

	requested := make(map[ID]commandRequest[ID])

	for {
		if events == nil && errs == nil {
			return
		}

		select {
		case err, ok := <-errs:
			if !ok {
				errs = nil
				break
			}
			outErrs <- fmt.Errorf("event stream: %w", err)
		case evt, ok := <-events:
			if !ok {
				events = nil
				break
			}

			b.debugLog("[subscribe] Handling %q event (%s)...", evt.Name(), evt.ID())

			switch evt.Name() {
			case CommandDispatched:
				data := evt.Data().(CommandDispatchedData[ID])

				if !containsName(names, data.Name) {
					break
				}

				load, err := b.enc.Decode(bytes.NewReader(data.Payload), data.Name)
				if err != nil {
					outErrs <- fmt.Errorf("decode payload: %w", err)
					break
				}

				cmd := command.New(
					data.ID,
					data.Name,
					load,
					command.Aggregate[any](data.AggregateName, data.AggregateID),
				)

				if err := b.requestCommand(ctx, cmd); err != nil {
					outErrs <- fmt.Errorf("request command: %w", err)
					break
				}

				requested[cmd.ID()] = commandRequest[ID]{
					cmd:  cmd,
					time: xtime.Now(),
				}

			case CommandAssigned:
				data := evt.Data().(CommandAssignedData[ID])

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
					outErrs <- fmt.Errorf("drop %q command: %w", req.cmd.Name(), ErrDrainTimeout)
				case out <- command.NewContext(
					context.Background(),
					req.cmd,
					command.WhenDone(func(ctx context.Context, cfg finish.Config) error {
						return b.markDone(ctx, req.cmd, cfg)
					}),
				):
				}
			}
		}
	}
}

func (b *Bus[ID]) newSubscriptionContext(parent context.Context, done <-chan struct{}) context.Context {
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

func (b *Bus[ID]) requestCommand(ctx context.Context, cmd command.Of[any, ID]) error {
	evt := event.New(b.newID().(ID), CommandRequested, CommandRequestedData[ID]{
		ID:        cmd.ID(),
		HandlerID: b.handlerID,
	})
	var err error
	b.debugMeasure(fmt.Sprintf("[subscribe] Publishing %q event", evt.Name()), func() {
		err = b.bus.Publish(ctx, evt.Any())
	})
	if err != nil {
		return fmt.Errorf("publish %q event: %w", evt.Name(), err)
	}
	return nil
}

func (b *Bus[ID]) acceptCommand(ctx context.Context, cmd command.Of[any, ID]) error {
	evt := event.New(b.newID().(ID), CommandAccepted, CommandAcceptedData[ID]{
		ID:        cmd.ID(),
		HandlerID: b.handlerID,
	})
	var err error
	b.debugMeasure(fmt.Sprintf("[subscribe] Publishing %q event", evt.Name()), func() {
		err = b.bus.Publish(ctx, evt.Any())
	})
	if err != nil {
		return fmt.Errorf("publish %q event: %w", evt.Name(), err)
	}
	return nil
}

func (b *Bus[ID]) markDone(ctx context.Context, cmd command.Of[any, ID], cfg finish.Config) error {
	var errmsg string
	if cfg.Err != nil {
		errmsg = cfg.Err.Error()
	}
	evt := event.New(b.newID().(ID), CommandExecuted, CommandExecutedData[ID]{
		ID:      cmd.ID(),
		Runtime: cfg.Runtime,
		Error:   errmsg,
	})
	var err error
	b.debugMeasure(fmt.Sprintf("[subscribe] Publishing %q event", evt.Name()), func() {
		err = b.bus.Publish(ctx, evt.Any())
	})
	if err != nil {
		return fmt.Errorf("publish %q event: %w", evt.Name(), err)
	}
	return nil
}

func (b *Bus[ID]) debugLog(format string, v ...any) {
	if b.logger != nil {
		b.logger.Printf(format+"\n", v...)
	}
}

func (b *Bus[ID]) debugMeasure(action string, fn func()) {
	start := time.Now()
	fn()

	if b.debug {
		end := time.Now()
		dur := end.Sub(start)
		b.debugLog("%s took %v (%v - %v).", action, dur, start, end)
	}
}

func containsName(names []string, name string) bool {
	for _, n := range names {
		if n == name {
			return true
		}
	}
	return false
}
