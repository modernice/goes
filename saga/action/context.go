package action

import (
	"context"
	"errors"

	"github.com/modernice/goes"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/command"
	"github.com/modernice/goes/command/cmdbus/dispatch"
	"github.com/modernice/goes/event"
)

var (
	// ErrMissingBus is returned when trying to publish an Event without an
	// event.Bus[ID] or dispatch a Command without a command.Bus[ID].
	ErrMissingBus = errors.New("missing bus")

	// ErrMissingRepository is returned when trying to fetch an Aggregate with
	// an .
	ErrMissingRepository = errors.New("missing repository")
)

// Context is the context for running actions.
type Context[ID goes.ID] interface {
	context.Context

	// Action returns the currently executed Action.
	Action() Action[ID]

	// Publish publishes the given Events via the underlying Event Bus. If no
	// Event Bus is available, Publish returns ErrMissingBus.
	Publish(context.Context, ...event.Of[any, ID]) error

	// Dispatch synchronously dispatches the given Command via the underlying
	// Command Bus. If no Command Bus is available, Dispatch returns ErrMissingBus.
	Dispatch(context.Context, command.Of[any, ID], ...command.DispatchOption) error

	// Fetch fetches the provided Aggregate from the underlying Aggregate
	// Repository. If no Aggregate Repository is available, Fetch returns
	// ErrMissingRepository.
	Fetch(context.Context, aggregate.AggregateOf[ID]) error

	// Run runs the Action with the specified name.
	Run(context.Context, string) error
}

// A ContextOption configures a Context.
type ContextOption[ID goes.ID] func(*actionContext[ID])

type actionContext[ID goes.ID] struct {
	context.Context

	act        Action[ID]
	eventBus   event.Bus[ID]
	commandBus command.Bus[ID]
	repo       aggregate.RepositoryOf[ID]
	run        func(context.Context, string) error
}

// WithEventBus returns a ContextOption that provides the Context with an
// event.Bus[ID].
func WithEventBus[ID goes.ID](bus event.Bus[ID]) ContextOption[ID] {
	return func(cfg *actionContext[ID]) {
		cfg.eventBus = bus
	}
}

// WithCommandBus returns a ContextOption that provides the Context with a
// command.Bus[ID].
func WithCommandBus[ID goes.ID](bus command.Bus[ID]) ContextOption[ID] {
	return func(cfg *actionContext[ID]) {
		cfg.commandBus = bus
	}
}

// WithRunner returns a ContextOption that specifies the runner function that is
// called when ctx.Run is called.
func WithRunner[ID goes.ID](run func(context.Context, string) error) ContextOption[ID] {
	return func(cfg *actionContext[ID]) {
		cfg.run = run
	}
}

// WithRepository returns a ContextOption that provides the Context with an .
func WithRepository[ID goes.ID](r aggregate.RepositoryOf[ID]) ContextOption[ID] {
	return func(cfg *actionContext[ID]) {
		cfg.repo = r
	}
}

// NewContext returns a new Context for the given Action.
func NewContext[ID goes.ID](parent context.Context, act Action[ID], opts ...ContextOption[ID]) Context[ID] {
	ctx := &actionContext[ID]{
		Context: parent,
		act:     act,
	}
	for _, opt := range opts {
		opt(ctx)
	}
	return ctx
}

func (ctx *actionContext[ID]) Action() Action[ID] {
	return ctx.act
}

func (ctx *actionContext[ID]) Publish(c context.Context, events ...event.Of[any, ID]) error {
	if ctx.eventBus == nil {
		return ErrMissingBus
	}
	return ctx.eventBus.Publish(c, events...)
}

func (ctx *actionContext[ID]) Dispatch(c context.Context, cmd command.Of[any, ID], opts ...command.DispatchOption) error {
	if ctx.commandBus == nil {
		return ErrMissingBus
	}
	opts = append([]command.DispatchOption{dispatch.Sync()}, opts...)
	return ctx.commandBus.Dispatch(c, cmd, opts...)
}

func (ctx *actionContext[ID]) Fetch(c context.Context, a aggregate.AggregateOf[ID]) error {
	if ctx.repo == nil {
		return ErrMissingRepository
	}
	return ctx.repo.Fetch(c, a)
}

func (ctx *actionContext[ID]) Run(c context.Context, name string) error {
	if ctx.run == nil {
		return nil
	}
	return ctx.run(c, name)
}
