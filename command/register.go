package command

import (
	"fmt"

	"github.com/modernice/goes/helper/pick"
)

// A Registerer is an object that can register handlers for different commands.
type Registerer interface {
	// RegisterCommandHandler registers a command handler for the given command name.
	RegisterCommandHandler(commandName string, handler func(Context) error)
}

// RegisterHandler registers command handler for the given command name.
// The provided Registerer should usually be an aggregate that uses the
// registered handlers to execute its own aggregate commands.
//
// Handler is implemented by
//  - *command.AggregateHandler
//
//	type Foo struct {
//		*aggregate.Base
//		*command.AggregateHandler
//
//		Foo string
//		Bar string
//		Baz string
//	}
//
//	type FooEvent { Foo string }
//	type BarEvent { Bar string }
//	type BazEvent { Bar string }
//
//	func NewFoo(id uuid.UUID) *Foo  {
//		foo := &Foo{
//			Base: aggregate.New("foo", id),
//          aggregateHandler: command.NewAggregateHandler(),
//		}
//
//	    // Register event appliers.
//		event.ApplyWith(foo, foo.foo, "foo")
//		event.ApplyWith(foo, foo.bar, "bar")
//		event.ApplyWith(foo, foo.baz, "baz")
//
//      // Register command handlers.
//		command.HandleWith(foo, func(ctx command.Ctx[string]) {
//			return foo.Foo(ctx.Payload())
//		}, "foo")
//		command.HandleWith(foo, func(ctx command.Ctx[string]) {
//			return foo.Bar(ctx.Payload())
//		}, "bar")
//		command.HandleWith(foo, func(ctx command.Ctx[string]) {
//			return foo.Baz(ctx.Payload())
//		}, "baz")
//
//		return foo
//	}
//
//	func (f *Foo) Foo(input string) error {
//		aggregate.Next(f, "foo", FooEvent{Foo: input})
//  }
//
//	func (f *Foo) foo(e event.Of[FooEvent]) {
//		f.Foo = e.Data().Foo
//	}
//
//	func (f *Foo) Bar(input string) error {
//		aggregate.Next(f, "bar", BarEvent{Bar: input})
//  }
//
//	func (f *Foo) bar(e event.Of[BarEvent]) {
//		f.Bar = e.Data().Bar
//	}
//
//	func (f *Foo) Baz(input string) error {
//		aggregate.Next(f, "baz", BazEvent{Baz: input})
//  }
//
//	func (f *Foo) baz(e event.Of[BazEvent]) {
//		f.Baz = e.Data().Baz
//	}
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
