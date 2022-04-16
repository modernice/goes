package handler

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/command"
	"github.com/modernice/goes/helper/streams"
)

// HandlerBase can be embedded into an aggregate to implement the aggregate
// interface. Provided methods are
//	- RegisterCommandHandler(string, func(command.Context) error)
//	- CommandNames() []string
//	- HandleCommand(command.Context) error
//
// HandlerBase allows to do the following:
//
//	type MyAggregate struct {
//    *aggregate.Base
//    *handler.HandlerBase
//  }
//
//	func New(id uuid.UUID) *MyAggregate {
//		foo := &MyAggregate{
//			Base: aggregate.NewBase("foo", id),
//			HandlerBase: handler.NewBase(),
//		}
//
//		// Handle "foo" and "bar" commands using the provided handler function.
//		command.HandelWith(foo, func(ctx command.Ctx[string]) error { ... }, "foo", "bar")
//
//		return foo
//	}
//
// HandlerBase is not named Base to avoid name collisions with the
// aggregate.Base type.
type HandlerBase struct {
	handlers map[string]func(command.Context) error
}

// NewBase returns a new *HandlerBase that can be embedded into an aggregate to
// implement the aggregate interface.
func NewBase() *HandlerBase {
	return &HandlerBase{
		handlers: make(map[string]func(command.Context) error),
	}
}

// RegisterHandler registers a command handler for the given command name.
func (base *HandlerBase) RegisterCommandHandler(commandName string, handler func(command.Context) error) {
	base.handlers[commandName] = handler
}

// CommandNames returns the commands that this handler (usually an aggregate) handles.
func (base *HandlerBase) CommandNames() []string {
	names := make([]string, 0, len(base.handlers))
	for name := range base.handlers {
		names = append(names, name)
	}
	return names
}

// HandleCommand executes the command handler on the given command.
func (base *HandlerBase) HandleCommand(ctx command.Context) error {
	if handler, ok := base.handlers[ctx.Name()]; ok {
		return handler(ctx)
	}
	return fmt.Errorf("no handler registered for %q command", ctx.Name())
}

// Aggregate is an aggregate that handles commands by itself.
type Aggregate interface {
	aggregate.Aggregate

	// CommandNames returns the commands that the aggregate handles.
	CommandNames() []string

	// HandleCommand executes the command handler of the given command.
	HandleCommand(command.Context) error
}

// Of is a command handler for a specific aggregate. It subscribes to the
// commands of the aggregate and calls its registered command handlers.
// It is important that the provided newFunc that instantiates the aggregates
// has no side effects other than the setup of the aggregate, because in order
// to know which commands are handled by the aggregate, Of.Handle() initially
// creates an instance of the aggregate using the provided newFunc with a random
// UUID and calls CommandNames() on it to extract the command names from the
// registered handlers.
type Of[A Aggregate] struct {
	handler *command.Handler[any]
	repo    aggregate.Repository
	newFunc func(uuid.UUID) A
}

// New returns a new command handler for commands of the given aggregate type
// that are published over the provided bus. Commands are handled by the
// aggregate itself, using the HandleCommand() method of the aggregate.
// The provided newFunc is used to instantiate the aggregates and to initially
// extract from the aggregate which commands it handles.
//
// Under the hood, a generic *command.Handler is used.
func New[A Aggregate](newFunc func(uuid.UUID) A, repo aggregate.Repository, bus command.Bus) *Of[A] {
	if newFunc == nil {
		panic("[goes/command.NewHandlerOf] newFunc is nil")
	}

	if repo == nil {
		panic("[goes/command.NewHandlerOf] repository is nil")
	}

	if bus == nil {
		panic("[goes/command.NewHandlerOf] bus is nil")
	}

	return &Of[A]{
		handler: command.NewHandler[any](bus),
		repo:    repo,
		newFunc: newFunc,
	}
}

// MustHandle is like Handle but panics if there is an error.
func (h *Of[A]) MustHandle(ctx context.Context) <-chan error {
	errs, err := h.Handle(ctx)
	if err != nil {
		panic(fmt.Errorf("[goes/command/handler.Of@MustHandle] %w", err))
	}
	return errs
}

// Handle subscribes to and handles the commands for which a handler has been
// registered. Command errors are sent into the returned error channel.
func (h *Of[A]) Handle(ctx context.Context) (<-chan error, error) {
	names := h.newFunc(uuid.New()).CommandNames()

	var out []<-chan error
	for _, name := range names {
		errs, err := h.handler.Handle(ctx, name, func(ctx command.Context) error {
			a := h.newFunc(ctx.AggregateID())
			return h.repo.Use(ctx, a, func() error {
				return a.HandleCommand(ctx)
			})
		})
		if err != nil {
			return streams.FanInAll(out...), err
		}
		out = append(out, errs)
	}

	return streams.FanInAll(out...), nil
}
