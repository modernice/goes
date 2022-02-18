package event

import (
	"fmt"

	"github.com/modernice/goes"
)

// A Handler is an object that can register handlers for different events.
type Handler[ID goes.ID] interface {
	// RegisterHandler registers an event handler for the given event name.
	RegisterHandler(eventName string, handler func(Of[any, ID]))
}

// RegisterHandler registers an event handler for the given event name.
// The provided Handler should usually be an aggregate or projection that uses
// the registered handler to apply the events onto itself.
//
// Handler is implemented by
//  - aggregate.Base
//  - projection.Base
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
//		aggregate.Register(foo, "foo", foo.foo)
//		aggregate.Register(foo, "bar", foo.bar)
//		aggregate.Register(foo, "baz", foo.baz)
//		return foo
//	}
//
//	func (f *Foo) foo(e event.Of[FooEvent]) {
//		f.Foo = e.Data().Foo
//	}
//
//	func (f *Foo) foo(e event.Of[BarEvent]) {
//		f.Bar = e.Data().Bar
//	}
//
//	func (f *Foo) foo(e event.Of[BazEvent]) {
//		f.Baz = e.Data().Baz
//	}
func RegisterHandler[Data any, ID goes.ID, Event Of[Data, ID]](eh Handler[ID], eventName string, handler func(Event)) {
	eh.RegisterHandler(eventName, func(evt Of[any, ID]) {
		if casted, ok := TryCast[Data](evt); ok {
			handler(any(casted).(Event))
		} else {
			var zero Data
			panic(fmt.Errorf(
				"[goes/event.RegisterHandler] Cannot cast %T to %T. "+
					"You probably provided the wrong event name for this handler. "+
					"[event=%v, handler=%T]",
				evt.Data(), zero, eventName, eh,
			))
		}
	})
}
