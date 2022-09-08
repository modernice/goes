package command

import (
	"fmt"

	"github.com/modernice/goes/helper/pick"
)

// A Registerer is a type that can register handlers for different commands.
type Registerer interface {
	// RegisterCommandHandler registers a command handler for the given command name.
	RegisterCommandHandler(commandName string, handler func(Context) error)
}

// Handlers is a map of event names to command handlers. Handlers can be embedded
// into structs to implement [Registerer].
type Handlers map[string]func(Context) error

// RegisterCommandHandler implements [Registerer].
func (h Handlers) RegisterCommandHandler(commandName string, handler func(Context) error) {
	h[commandName] = handler
}

// CommandHandlers returns the handlers for the given command.
func (h Handlers) CommandHandler(commandName string) func(Context) error {
	return h[commandName]
}

// HandleEvent calls the registered handler of the given [Command].
func (h Handlers) HandleCommand(ctx Context) error {
	if handler := h.CommandHandler(ctx.Name()); handler != nil {
		return handler(ctx)
	}
	return fmt.Errorf("no handler registered for command %q", ctx.Name())
}

// RegisterHandler registers the handler for the given command.
//
//		type Foo struct {
//			*aggregate.Base
//			*handler.BaseHandler
//
//			Foo string
//			Bar string
//			Baz string
//		}
//
//		type FooEvent { Foo string }
//		type BarEvent { Bar string }
//		type BazEvent { Bar string }
//
//		func NewFoo(id uuid.UUID) *Foo  {
//			foo := &Foo{
//				Base: aggregate.New("foo", id),
//	         Handler: handler.NewBase(),
//			}
//
//		    // Register event appliers.
//			event.ApplyWith(foo, foo.foo, "foo")
//			event.ApplyWith(foo, foo.bar, "bar")
//			event.ApplyWith(foo, foo.baz, "baz")
//
//	     // Register command handlers.
//			command.HandleWith(foo, func(ctx command.Ctx[string]) {
//				return foo.Foo(ctx.Payload())
//			}, "foo")
//			command.HandleWith(foo, func(ctx command.Ctx[string]) {
//				return foo.Bar(ctx.Payload())
//			}, "bar")
//			command.HandleWith(foo, func(ctx command.Ctx[string]) {
//				return foo.Baz(ctx.Payload())
//			}, "baz")
//
//			return foo
//		}
//
//		func (f *Foo) Foo(input string) error {
//			aggregate.Next(f, "foo", FooEvent{Foo: input})
//	 }
//
//		func (f *Foo) foo(e event.Of[FooEvent]) {
//			f.Foo = e.Data().Foo
//		}
//
//		func (f *Foo) Bar(input string) error {
//			aggregate.Next(f, "bar", BarEvent{Bar: input})
//	 }
//
//		func (f *Foo) bar(e event.Of[BarEvent]) {
//			f.Bar = e.Data().Bar
//		}
//
//		func (f *Foo) Baz(input string) error {
//			aggregate.Next(f, "baz", BazEvent{Baz: input})
//	 }
//
//		func (f *Foo) baz(e event.Of[BazEvent]) {
//			f.Baz = e.Data().Baz
//		}
func RegisterHandler[Payload any](r Registerer, commandName string, handler func(Ctx[Payload]) error) {
	r.RegisterCommandHandler(commandName, func(ctx Context) error {
		if casted, ok := TryCastContext[Payload](ctx); ok {
			return handler(casted)
		}

		aggregateName := "<unknown>"
		if a, ok := r.(pick.AggregateProvider); ok {
			aggregateName = pick.AggregateName(a)
		}
		var zero Payload
		panic(fmt.Errorf(
			"[goes/command.RegisterHandler] Cannot cast %T to %T. "+
				"You probably provided the wrong command name for this handler. "+
				"[command=%v, aggregate=%v]",
			ctx.Payload(), zero, commandName, aggregateName,
		))
	})
}

// HandleWith is an alias for RegisterHandler.
func HandleWith[Payload any](r Registerer, handler func(Ctx[Payload]) error, commandNames ...string) {
	if len(commandNames) == 0 {
		panic("command.HandleWith: no command names provided")
	}
	for _, name := range commandNames {
		RegisterHandler(r, name, handler)
	}
}

// ApplyWith registers the command applier for the given commands. When a
// command handler one of the given commands, it calls the provided apply
// function with the command payload to execute the command.
//
// ApplyWith calls HandleWith under the hood to register the command handler.
func ApplyWith[Payload any](r Registerer, apply func(Payload) error, commandNames ...string) {
	HandleWith(r, func(ctx Ctx[Payload]) error {
		return apply(ctx.Payload())
	}, commandNames...)
}
