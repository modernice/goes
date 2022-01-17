package action

import (
	"context"
	"errors"

	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/command"
	"github.com/modernice/goes/command/cmdbus/dispatch"
	"github.com/modernice/goes/event"
)

var (
	// ErrMissingBus is returned when trying to publish an Event without an
	// event.Bus or dispatch a Command without a command.Bus.
	ErrMissingBus = errors.New("missing bus")

	// ErrMissingRepository is returned when trying to fetch an Aggregate with
	// an .
	ErrMissingRepository = errors.New("missing repository")
)

// Context is the context for running actions.
type Context[E, C any] interface {
	context.Context

	// Action returns the currently executed Action.
	Action() Action[E, C]

	// Publish publishes the given Events via the underlying Event Bus. If no
	// Event Bus is available, Publish returns ErrMissingBus.
	Publish(context.Context, ...event.Event[E]) error

	// Dispatch synchronously dispatches the given Command via the underlying
	// Command Bus. If no Command Bus is available, Dispatch returns ErrMissingBus.
	Dispatch(context.Context, command.Command[C], ...command.DispatchOption) error

	// Fetch fetches the provided Aggregate from the underlying Aggregate
	// Repository. If no Aggregate Repository is available, Fetch returns
	// ErrMissingRepository.
	Fetch(context.Context, aggregate.Aggregate[E]) error

	// Run runs the Action with the specified name.
	Run(context.Context, string) error
}

// A ContextOption configures a Context.
type ContextOption[E, C any] func(*actionContext[E, C])

type actionContext[E, C any] struct {
	context.Context

	act        Action[E, C]
	eventBus   event.Bus[E]
	commandBus command.Bus[C]
	repo       aggregate.Repository[E]
	run        func(context.Context, string) error
}

// WithEventBus returns a ContextOption that provides the Context with an
// event.Bus.
func WithEventBus[E, C any](bus event.Bus[E]) ContextOption[E, C] {
	return func(cfg *actionContext[E, C]) {
		cfg.eventBus = bus
	}
}

// WithCommandBus returns a ContextOption that provides the Context with a
// command.Bus.
func WithCommandBus[E, C any](bus command.Bus[C]) ContextOption[E, C] {
	return func(cfg *actionContext[E, C]) {
		cfg.commandBus = bus
	}
}

// WithRunner returns a ContextOption that specifies the runner function that is
// called when ctx.Run is called.
func WithRunner[E, C any](run func(context.Context, string) error) ContextOption[E, C] {
	return func(cfg *actionContext[E, C]) {
		cfg.run = run
	}
}

// WithRepository returns a ContextOption that provides the Context with an .
func WithRepository[E, C any](r aggregate.Repository[E]) ContextOption[E, C] {
	return func(cfg *actionContext[E, C]) {
		cfg.repo = r
	}
}

// NewContext returns a new Context for the given Action.
func NewContext[E, C any](parent context.Context, act Action[E, C], opts ...ContextOption[E, C]) Context[E, C] {
	ctx := &actionContext[E, C]{
		Context: parent,
		act:     act,
	}
	for _, opt := range opts {
		opt(ctx)
	}
	return ctx
}

func (ctx *actionContext[E, C]) Action() Action[E, C] {
	return ctx.act
}

func (ctx *actionContext[E, C]) Publish(c context.Context, events ...event.Event[E]) error {
	if ctx.eventBus == nil {
		return ErrMissingBus
	}
	return ctx.eventBus.Publish(c, events...)
}

func (ctx *actionContext[E, C]) Dispatch(c context.Context, cmd command.Command[C], opts ...command.DispatchOption) error {
	if ctx.commandBus == nil {
		return ErrMissingBus
	}
	opts = append([]command.DispatchOption{dispatch.Sync()}, opts...)
	return ctx.commandBus.Dispatch(c, cmd, opts...)
}

func (ctx *actionContext[E, C]) Fetch(c context.Context, a aggregate.Aggregate[E]) error {
	if ctx.repo == nil {
		return ErrMissingRepository
	}
	return ctx.repo.Fetch(c, a)
}

func (ctx *actionContext[E, C]) Run(c context.Context, name string) error {
	if ctx.run == nil {
		return nil
	}
	return ctx.run(c, name)
}
