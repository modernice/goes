package command

import (
	"context"

	"github.com/google/uuid"
	"github.com/modernice/goes/command/cmdbus/report"
	"github.com/modernice/goes/command/finish"
)

// Bus is the pub-sub client for commands.
type Bus interface {
	Dispatcher
	Subscriber
}

// A Dispatcher dispatches commands to subscribed handlers.
type Dispatcher interface {
	// Dispatch dispatches the provided command to subscribers of the command.
	//
	// Depending on the implementation, the command may be dispatched to a
	// single or multiple subscribers. The implementation provided by the
	// `cmdbus` package ensures that a command is dispatched to at most one
	// subscriber.
	Dispatch(context.Context, Command, ...DispatchOption) error
}

// A Subscriber subscribes to commands that are dispatched by a Dispatcher.
type Subscriber interface {
	// Subscribe subscribes to the given commands and returns two channels –
	// a Context channel and an error channel. When a command is dispatched to
	// this subscriber, a new Context is sent into the Context channel. Context
	// is a context.Context that additionally provides the dispatched command data.
	//
	//	var bus command.Bus
	//	// Subscribe to "foo" and "bar" commands.
	//	res, errs, err := bus.Subscribe(context.TODO(), "foo", "bar")
	//	// handle err
	//	for ctx := range res {
	//		log.Printf(
	//			"Handling %q command ... [aggregate_name: %q, aggregate_id: %q]",
	//			ctx.Name(), ctx.AggregateName(), ctx.AggregateID(),
	//		)
	//
	//	    log.Printf("Payload:\n%v", ctx.Payload())
	//	}
	Subscribe(ctx context.Context, names ...string) (<-chan Context, <-chan error, error)
}

// DispatchOption is an option for dispatching commands.
type DispatchOption func(*DispatchConfig)

// Config is the configuration for dispatching a command.
type DispatchConfig struct {
	// A synchronous dispatch waits for the execution of the Command to finish
	// and returns the execution error if there was any.
	//
	// A dispatch is automatically made synchronous when Repoter is non-nil.
	Synchronous bool

	// If Reporter is not nil, the Bus will report the execution result of a
	// Command to Reporter by calling Reporter.Report().
	//
	// A non-nil Reporter makes the dispatch synchronous.
	Reporter Reporter
}

// A Reporter reports execution results of a Command.
type Reporter interface {
	Report(report.Report)
}

// Context is the context of a dispatched command with an arbitrary payload.
type Context = Ctx[any]

// Ctx is the context of a dispatched command with a specific payload type.
type Ctx[P any] interface {
	context.Context
	Of[P]

	// AggregateID returns the id of the aggregate that the command is linked to,
	// or uuid.Nil if no aggregate was linked to this command.
	AggregateID() uuid.UUID

	// AggregateName returns the name of the aggregate that the command is linked to,
	// or "" if no aggregate was linked to this command.
	AggregateName() string

	// Finish should be called after the command has been handled to notify the
	// dispatcher bus about the execution result. If a command was dispatched
	// synchronously, Finish must be called; otherwise the dispatcher bus will
	// never return.
	Finish(context.Context, ...finish.Option) error
}
