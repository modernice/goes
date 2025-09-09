package event

import (
	"fmt"

	"github.com/modernice/goes/helper/pick"
)

// Registerer registers handlers for event names. It is accepted by helper functions like RegisterHandler, ApplyWith and HandleWith.
type Registerer interface {
	// RegisterEventHandler registers an event handler for the given event name.
	RegisterEventHandler(eventName string, handler func(Event))
}

// Handlers maps event names to callbacks and implements Registerer. Embedding it allows types to conveniently register handlers.
type Handlers map[string][]func(Event)

// RegisterEventHandler implements Registerer.
func (h Handlers) RegisterEventHandler(eventName string, handler func(Event)) {
	h[eventName] = append(h[eventName], handler)
}

// EventHandlers returns handlers for eventName.
func (h Handlers) EventHandlers(eventName string) []func(Event) {
	return h[eventName]
}

// HandleEvent invokes all handlers for evt.
func (h Handlers) HandleEvent(evt Event) {
	handlers := h.EventHandlers(evt.Name())
	for _, handler := range handlers {
		handler(evt)
	}
}

// RegisterHandler registers handler for eventName.
//
// Example using *aggregate.Base:
//
//	type Foo struct {
//		*aggregate.Base
//
//		Foo string
//		Bar int
//		Baz bool
//	}
//
//	type FooEvent { Foo string }
//	type BazEvent { Baz bool }
//
//	func NewFoo(id uuid.UUID) *Foo  {
//		foo := &Foo{Base: aggregate.New("foo", id)}
//
//		event.RegisterHandler(foo, "foo", foo.applyFoo)
//		event.RegisterHandler(foo, "bar", foo.applyBar)
//		event.RegisterHandler(foo, "baz", foo.applyBaz)
//
//		return foo
//	}
//
//	func (f *Foo) applyFoo(e event.Of[FooEvent]) {
//		f.Foo = e.Data().Foo
//	}
//
//	func (f *Foo) applyBar(e event.Of[int]) {
//		f.Bar = e.Data()
//	}
//
//	func (f *Foo) applyBaz(e event.Of[BazEvent]) {
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

// ApplyWith registers handler for all eventNames. It is an alias for RegisterHandler:
//
//	type Foo struct {
//		*projection.Base
//
//		Foo string
//	}
//
//	func NewFoo() *Foo  {
//		foo := &Foo{Base: projection.New()}
//
//		// Because we "apply" events to the projection.
//		event.ApplyWith(foo, foo.applyFoo, "foo")
//
//		return foo
//	}
//
//	func (f *Foo) applyFoo(e event.Of[string]) {
//		f.Foo = e.Data()
//	}
func ApplyWith[Data any](r Registerer, handler func(Of[Data]), eventNames ...string) {
	for _, name := range eventNames {
		RegisterHandler(r, name, handler)
	}
}

// HandleWith registers handler for all eventNames. It is an alias for RegisterHandler:
//
//	import "github.com/modernice/goes/event/handler"
//
//	var bus event.Bus
//	h := handler.New(bus)
//
//	event.HandleWith(h, h.handleFoo, "foo")
func HandleWith[Data any](r Registerer, handler func(Of[Data]), eventNames ...string) {
	for _, name := range eventNames {
		RegisterHandler(r, name, handler)
	}
}
