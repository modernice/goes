// Package cmdbus provides a distributed & event-driven Command Bus.
package cmdbus

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	commandpb "github.com/modernice/goes/api/proto/gen/command"
	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/command"
	"github.com/modernice/goes/command/cmdbus/dispatch"
	"github.com/modernice/goes/command/cmdbus/report"
	"github.com/modernice/goes/command/finish"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/handler"
	"github.com/modernice/goes/helper/streams"
	"github.com/modernice/goes/internal"
	"github.com/modernice/goes/internal/concurrent"
	"golang.org/x/exp/constraints"
	"google.golang.org/protobuf/proto"
)

var _ command.Bus = (*Bus[int])(nil)

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

	// // ErrNotRunning is returned when trying to dispatch or subscribe to a
	// // command before the command bus has been started.
	// ErrNotRunning = errors.New("command bus is not running")

	// ErrAlreadySubscribed is returned when trying to subscribe to the same
	// commands more than once within a single command bus.
	ErrSubscribed = errors.New("already subscribed to command")
)

// Bus is an event-driven Command Bus.
type Bus[ErrorCode constraints.Integer] struct {
	*handler.Handler

	options

	subMux        sync.RWMutex
	subscriptions map[string]*subscription
	requested     map[uuid.UUID]command.Cmd[any]

	dispatchMux sync.RWMutex
	dispatched  map[uuid.UUID]dispatcher
	assigned    map[uuid.UUID]dispatcher

	enc codec.Encoding
	bus event.Bus
	id  uuid.UUID

	errs chan error
	fail func(error)
}

type options struct {
	assignTimeout  time.Duration
	receiveTimeout time.Duration
	filters        []func(command.Command) bool
	debug          bool
	workers        int
}

type subscription struct {
	commands chan command.Context
	errs     chan error
}

type dispatcher struct {
	cmd             command.Command
	cfg             command.DispatchConfig
	accepted        chan struct{}
	dispatchAborted chan struct{}
	out             chan error
}

// Option is a command bus option.
type Option func(*options)

// AssignTimeout returns an Option that configures the timeout when assigning a
// Command to a Handler. A zero Duration means no timeout.
//
// A zero Duration means no timeout. The default timeout is 5s.
func AssignTimeout(dur time.Duration) Option {
	return func(opts *options) {
		opts.assignTimeout = dur
	}
}

// ReceiveTimeout returns an Option that configures the timeout for receiving a
// command context from the command bus. If the command is not received from the
// returned channel within the configured timeout, the command is dropped.
//
// A zero Duration means no timeout. The default timeout is 10s.
func ReceiveTimeout(dur time.Duration) Option {
	return func(opts *options) {
		opts.receiveTimeout = dur
	}
}

// Deprecated: Use ReceiveTimeout instead.
func DrainTimeout(dur time.Duration) Option {
	return ReceiveTimeout(dur)
}

// Debug returns an Option that toggles the debug mode of the command bus.
func Debug(debug bool) Option {
	return func(opts *options) {
		opts.debug = debug
	}
}

// Workers returns an Option that configures how many commands are processed in
// parallel by the command bus. Internally, this sets the worker count of the
// underlying event handler that processes command bus events. If n < 1, the
// handler defaults to 1 worker.
func Workers(n int) Option {
	return func(opts *options) {
		opts.workers = n
	}
}

// Filter returns an Option that adds a filter to the command bus. Filters allow
// you to restrict the commands that are handled by the bus: By default, the bus
// handles all commands that it's subscribed to. Filters are called before the
// bus assigns itself as the handler of a command. If any of the registered
// filters returns false for a command, the command will not be handled by the
// bus.
func Filter(fn func(command.Command) bool) Option {
	return func(opts *options) {
		opts.filters = append(opts.filters, fn)
	}
}

// New returns an event-driven command bus.
func New[ErrorCode constraints.Integer](enc codec.Encoding, events event.Bus, opts ...Option) *Bus[ErrorCode] {
	b := &Bus[ErrorCode]{
		options: options{
			assignTimeout:  DefaultAssignTimeout,
			receiveTimeout: DefaultReceiveTimeout,
		},
		subscriptions: make(map[string]*subscription),
		requested:     make(map[uuid.UUID]command.Cmd[any]),
		dispatched:    make(map[uuid.UUID]dispatcher),
		assigned:      make(map[uuid.UUID]dispatcher),
		enc:           enc,
		bus:           events,
		id:            internal.NewUUID(),
	}
	for _, opt := range opts {
		opt(&b.options)
	}

	// create the underlying event handler with the configured worker count
	b.Handler = handler.New(events, handler.Workers(b.options.workers))

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
func (b *Bus[ErrorCode]) Run(ctx context.Context) (<-chan error, error) {
	b.debugLog("starting command bus ...")

	errs, err := b.Handler.Run(ctx)
	if err != nil {
		return errs, err
	}

	b.errs, b.fail = concurrent.Errors(ctx)
	out, _ := streams.FanIn(b.errs, errs)

	b.debugLog("command bus started ...")

	return out, nil
}

// Dispatch dispatches a Command to the appropriate handler (Command Bus) using
// the underlying event Bus to communicate between b and the other Command Buses.
//
// # How it works
//
// Dispatch first publishes a CommandDispatched event with the Command Payload
// encoded in the event Data. Every Command Bus that is currently subscribed to
// a Command receives the CommandDispatched event and checks if it handles
// Commands that have the name of the dispatched Command.
//
// If a Command Bus doesn't handle Commands with that name, they just ignore the
// CommandDispatched event, but if they're instructed to handle such Commands,
// they tell the Bus b that they want to handle the Command by publishing a
// CommandRequested event which the Bus b will listen for.
//
// The first of those CommandRequested events that the Bus b receives is used to
// assign the Command to a Handler. When b receives the first CommandRequested
// Event, it publishes a CommandAssigned event with the ID of the selected
// Handler.
//
// The handler Command Buses receive the CommandAssigned event and check if
// they're Handler that is assigned to the Command. The assigned Handler then
// publishes a final CommandAccepted event to tell the Bus b that the Command
// arrived at its Handler.
//
// # Errors
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
// # Execution result
//
// By default, Dispatch does not return information about the execution of a
// Command, but a report.Reporter can be provided with the dispatch.Report()
// Option. When a Reporter is provided, the dispatch is automatically made
// synchronous.
//
// Example:
//
//	var rep report.Report
//	var cmd command.Command
//	err := b.Dispatch(context.TODO(), cmd, dispatch.Report(&rep))
//	log.Println(fmt.Sprintf("Command: %v", rep.Command()))
//	log.Println(fmt.Sprintf("Runtime: %v", rep.Runtime()))
//	log.Println(fmt.Sprintf("Error: %v", err))
func (b *Bus[ErrorCode]) Dispatch(ctx context.Context, cmd command.Command, opts ...command.DispatchOption) (err error) {
	b.debugLog("dispatching %q command ...", cmd.Name())

	if !b.Running() {
		errs, err := b.Run(context.Background())
		if err != nil {
			return err
		}

		b.debugLog("logging errors from command bus to stderr ...")
		go logErrors(errs)
	}

	cfg := dispatch.Configure(opts...)

	load, err := b.enc.Marshal(cmd.Payload())
	if err != nil {
		return fmt.Errorf("encode payload: %w", err)
	}

	id, name := cmd.Aggregate().Split()

	evt := event.New(CommandDispatched, CommandDispatchedData{
		ID:            cmd.ID(),
		Name:          cmd.Name(),
		AggregateName: name,
		AggregateID:   id,
		Payload:       load,
	})

	out := make(chan error)
	accepted := make(chan struct{})
	aborted := make(chan struct{})
	defer close(aborted)

	b.dispatchMux.Lock()
	b.dispatched[cmd.ID()] = dispatcher{
		cmd:             cmd,
		cfg:             cfg,
		accepted:        accepted,
		out:             out,
		dispatchAborted: aborted,
	}
	b.dispatchMux.Unlock()

	defer b.cleanupDispatch(cmd.ID())

	b.debugLog("publishing %q event ...", evt.Name())

	if err := b.bus.Publish(ctx, evt.Any()); err != nil {
		return fmt.Errorf("publish %q event: %w", evt.Name(), err)
	}

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

func (b *Bus[ErrorCode]) cleanupDispatch(cmdID uuid.UUID) {
	b.dispatchMux.Lock()
	defer b.dispatchMux.Unlock()
	delete(b.dispatched, cmdID)
	delete(b.assigned, cmdID)
}

// Subscribe returns a channel of Command Contexts and an error channel. The
// Context channel channel is registered as a handler for Commands which have
// one of the specified names.
//
// Callers of Subscribe are responsible for receiving from the returned error
// channel to prevent a deadlock.
//
// When a Command Bus, which uses the same underlying event Bus as Bus b,
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
func (b *Bus[ErrorCode]) Subscribe(ctx context.Context, names ...string) (<-chan command.Ctx[any], <-chan error, error) {
	if !b.Running() {
		errs, err := b.Run(context.Background())
		if err != nil {
			return nil, nil, err
		}

		b.debugLog("logging errors from command bus to stderr ...")
		go logErrors(errs)
	}

	out, errs := make(chan command.Context), make(chan error)

	if len(names) == 0 {
		return out, errs, nil
	}

	b.subMux.Lock()
	defer b.subMux.Unlock()

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
		b.subMux.Lock()
		defer b.subMux.Unlock()

		close(out)
		close(errs)

		for _, name := range names {
			delete(b.subscriptions, name)
		}
	}()

	return out, errs, nil
}

func (b *Bus[ErrorCode]) commandDispatched(evt event.Of[CommandDispatchedData]) {
	data := evt.Data()

	load, err := b.enc.Unmarshal(data.Payload, data.Name)
	if err != nil {
		b.fail(fmt.Errorf("[goes/command/cmdbus.Bus@commandDispatched] Failed to decode %q command: %w\n%s", data.Name, err, data.Payload))
		return
	}

	// if the bus does not handle the dispatched command, return
	if !b.handles(data.Name) {
		return
	}

	cmd := command.New(data.Name, load, command.ID(data.ID), command.Aggregate(data.AggregateName, data.AggregateID))

	// apply user-defined filters
	if !b.filterAllows(cmd) {
		return
	}

	// otherwise request to become the handler of the command
	requestEvent := event.New(CommandRequested, CommandRequestedData{
		ID:    data.ID,
		BusID: b.id,
	})

	b.debugLog("requesting to become the handler for %q command ... [id=%s]", data.Name, data.ID)
	b.debugLog("publishing %q event ...", requestEvent.Name())

	if err := b.bus.Publish(b.Context(), requestEvent.Any()); err != nil {
		b.fail(fmt.Errorf("[goes/command/cmdbus.Bus@commandDispatched] Failed to request %q command: %w", data.Name, err))
		return
	}

	b.requested[data.ID] = cmd
}

func (b *Bus[ErrorCode]) handles(name string) bool {
	b.debugLog("checking if %q command is handled by this bus ...", name)
	b.subMux.RLock()
	defer b.subMux.RUnlock()
	if _, ok := b.subscriptions[name]; !ok {
		b.debugLog("this bus does not handle %q commands", name)
		return false
	}
	b.debugLog("this bus handles %q commands", name)
	return true
}

func (b *Bus[ErrorCode]) filterAllows(cmd command.Command) bool {
	for _, fn := range b.filters {
		if !fn(cmd) {
			b.debugLog("filtered out %q command", cmd.Name())
			return false
		}
	}
	return true
}

func (b *Bus[ErrorCode]) commandRequested(evt event.Of[CommandRequestedData]) {
	data := evt.Data()

	// if the bus did not dispatch the command, return
	b.dispatchMux.RLock()
	cmd, ok := b.dispatched[data.ID]
	b.dispatchMux.RUnlock()
	if !ok {
		return
	}

	b.dispatchMux.Lock()
	defer b.dispatchMux.Unlock()

	// otherwise remove the command from the dispatched commands
	delete(b.dispatched, data.ID)

	// and assign the command to the handler that requested to handle it
	assignEvent := event.New(CommandAssigned, CommandAssignedData(data))

	b.debugLog("publishing %q event ...", assignEvent.Name())

	if err := b.bus.Publish(b.Context(), assignEvent.Any()); err != nil {
		b.fail(fmt.Errorf("[goes/command/cmdbus.Bus@commandRequested] Failed to assign %q command to handler %q: %w", cmd.cmd.Name(), data.BusID, err))
		return
	}

	// and add the command to the assigned commands
	b.assigned[data.ID] = cmd
}

func (b *Bus[ErrorCode]) commandAssigned(evt event.Of[CommandAssignedData]) {
	data := evt.Data()

	// if the bus did not request the command, return
	cmd, ok := b.requested[data.ID]
	if !ok {
		return
	}

	// otherwise remove the command from the requested commands
	delete(b.requested, data.ID)

	// if the command was assigned to another bus, return
	if b.id != data.BusID {
		return
	}

	// and accept the command
	acceptEvt := event.New(CommandAccepted, CommandAcceptedData(data))

	b.debugLog("publishing %q event ...", acceptEvt.Name())

	if err := b.bus.Publish(b.Context(), acceptEvt.Any()); err != nil {
		b.fail(fmt.Errorf("[goes/command/cmdbus.Bus@commandAssigned] Failed to accept %q command: %w", cmd.Name(), err))
		return
	}

	// then pass the command to the subscription
	b.subMux.Lock()
	defer b.subMux.Unlock()
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

func (b *Bus[ErrorCode]) markDone(ctx context.Context, cmd command.Command, cfg finish.Config) error {
	var errbytes []byte

	if cfg.Err != nil {
		// log.Printf("%#v", cfg.Err)
		cerr := command.Error[ErrorCode](cfg.Err)
		// log.Printf("%#v", cerr)
		msg := commandpb.NewError(cerr)
		// log.Printf("%#v", msg)
		b, err := proto.Marshal(msg)
		if err != nil {
			return fmt.Errorf("marshal command error: %w", err)
		}
		errbytes = b
	}

	evt := event.New(CommandExecuted, CommandExecutedData{
		ID:      cmd.ID(),
		Runtime: cfg.Runtime,
		Error:   errbytes,
	})

	b.debugLog("publishing %q event ...", evt.Name())

	if err := b.bus.Publish(ctx, evt.Any()); err != nil {
		return fmt.Errorf("publish %q event: %w", evt.Name(), err)
	}

	return nil
}

func (b *Bus[ErrorCode]) commandAccepted(evt event.Of[CommandAcceptedData]) {
	data := evt.Data()

	// if the bus did not assign the command, return
	b.dispatchMux.RLock()
	cmd, ok := b.assigned[data.ID]
	b.dispatchMux.RUnlock()
	if !ok {
		return
	}

	// otherwise mark the command as accepted
	select {
	case <-cmd.accepted:
	default:
		close(cmd.accepted)
	}

	// if the dispatch was not made synchronously, remove the command from
	// assigned commands, close the out channel and return
	if !cmd.cfg.Synchronous && cmd.cfg.Reporter == nil {
		b.dispatchMux.Lock()
		defer b.dispatchMux.Unlock()
		delete(b.assigned, data.ID)
		close(cmd.out)
	}
}

func (b *Bus[ErrorCode]) commandExecuted(evt event.Of[CommandExecutedData]) {
	data := evt.Data()

	// if the bus is not waiting for the execution of the command, return
	b.dispatchMux.RLock()
	cmd, ok := b.assigned[data.ID]
	b.dispatchMux.RUnlock()
	if !ok {
		return
	}

	// otherwise mark the command as accepted if it isn't already
	select {
	case <-cmd.accepted:
	default:
		close(cmd.accepted)
	}

	b.dispatchMux.Lock()
	defer b.dispatchMux.Unlock()

	// and remove the command from assigned commands
	delete(b.assigned, data.ID)

	// parse the command error
	var cmdError *command.Err[ErrorCode]
	if len(data.Error) > 0 {
		var errpb commandpb.Error
		if err := proto.Unmarshal(data.Error, &errpb); err != nil {
			err := fmt.Errorf("failed to unmarshal command error of %q command: %w", cmd.cmd.Name(), err)
			select {
			case <-b.Context().Done():
			case <-cmd.dispatchAborted:
			case cmd.out <- err:
			}
			return
		}
		cmdError = commandpb.AsError[ErrorCode](&errpb)
	}

	// if the dispatch requested a report, report the execution result
	if cmd.cfg.Reporter != nil {
		id, name := cmd.cmd.Aggregate().Split()

		cmd.cfg.Reporter.Report(report.New(report.Command{
			ID:            cmd.cmd.ID(),
			Name:          cmd.cmd.Name(),
			Payload:       cmd.cmd.Payload(),
			AggregateName: name,
			AggregateID:   id,
		}, report.Runtime(data.Runtime), report.Error(&ExecutionError[any]{
			Cmd: cmd.cmd,
			Err: cmdError,
		})))
	}

	// if command execution failed, send the error to the dispatcher error channel and return
	if cmdError != nil {
		select {
		case <-b.Context().Done():
			return
		case <-cmd.dispatchAborted:
		case cmd.out <- &ExecutionError[any]{
			Cmd: cmd.cmd,
			Err: cmdError,
		}:
		}
		return
	}

	// otherwise close the error channel of the dispatcher
	close(cmd.out)
}

func (b *Bus[ErrorCode]) debugLog(format string, vals ...any) {
	if b.debug {
		log.Printf("[goes/command/cmdbus.Bus@debugLog] "+format, vals...)
	}
}

// logging errors to stderr if the command bus was started by Dispatch() or Subscribe().
func logErrors(errs <-chan error) {
	for err := range errs {
		if err != nil {
			log.Printf("[goes/command/cmdbus.logErrors] %v", err)
		}
	}
}
