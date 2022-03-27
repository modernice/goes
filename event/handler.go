package event

import (
	"fmt"

	"github.com/modernice/goes/helper/pick"
)

// A Registerer is an object that can register handlers for different events.
type Registerer interface {
	// RegisterEventHandler registers an event handler for the given event name.
	RegisterEventHandler(eventName string, handler func(Event))
}

// RegisterHandler registers an event handler for the given event name.
// The provided Registerer should usually be an aggregate or projection that
// uses the registered handlers to apply events onto itself.
//
// Registerer is implemented by
//  - *aggregate.Base
//  - *projection.Base
//
//	type Foo struct {
//		*aggregate.Base
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
//		foo := &Foo{Base: aggregate.New("foo", id)}
//		event.ApplyWith(foo, foo.foo, "foo")
//		event.ApplyWith(foo, foo.bar, "bar")
//		event.ApplyWith(foo, foo.baz, "baz")
//		return foo
//	}
//
//	func (f *Foo) foo(e event.Of[FooEvent]) {
//		f.Foo = e.Data().Foo
//	}
//
//	func (f *Foo) bar(e event.Of[BarEvent]) {
//		f.Bar = e.Data().Bar
//	}
//
//	func (f *Foo) baz(e event.Of[BazEvent]) {
//		f.Baz = e.Data().Baz
//	}
func RegisterHandler[Data any](r Registerer, eventName string, handler func(Of[Data])) {
	r.RegisterEventHandler(eventName, func(evt Event) {
		if casted, ok := TryCast[Data](evt); ok {
			handler(casted)
			return
		}

		aggregateName := "<unknown>"
		if a, ok := r.(pick.AggregateProvider); ok {
			aggregateName = pick.AggregateName(a)
		}
		var zero Data
		panic(fmt.Errorf(
			"[goes/event.RegisterHandler] Cannot cast %T to %T. "+
				"You probably provided the wrong event name for this handler. "+
				"[event=%v, aggregate=%v]",
			evt.Data(), zero, eventName, aggregateName,
		))
	})
}

// ApplyWith is an alias for RegisterHandler.
func ApplyWith[Data any](r Registerer, handler func(Of[Data]), eventNames ...string) {
	if len(eventNames) == 0 {
		panic("event.ApplyWith: no event names provided")
	}
	for _, name := range eventNames {
		RegisterHandler(r, name, handler)
	}
}

// HandleWith is an alias for RegisterHandler.
func HandleWith[Data any](r Registerer, handler func(Of[Data]), eventNames ...string) {
	if len(eventNames) == 0 {
		panic("event.HandleWith: no event names provided")
	}
	for _, name := range eventNames {
		RegisterHandler(r, name, handler)
	}
}
