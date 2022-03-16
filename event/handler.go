package event

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/modernice/goes/helper/pick"
	"github.com/modernice/goes/helper/streams"
	"github.com/modernice/goes/internal/concurrent"
)

// ErrRunning is returned when trying to run a *Handler that is already running.
var ErrRunning = errors.New("event handler is already running")

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

// A Handler asynchronously handles published events.
// Use NewHandler to create a Handler.
type Handler struct {
	bus        Bus
	handlers   map[string]func(Event)
	eventNames map[string]struct{}

	mux sync.RWMutex
	ctx context.Context
}

// NewHandler returns an event handler for published events.
func NewHandler(bus Bus) *Handler {
	return &Handler{
		bus:        bus,
		handlers:   make(map[string]func(Event)),
		eventNames: make(map[string]struct{}),
	}
}

// RegisterEventHandler registers the handler for the given event.
// Events must be registered before h.Run() is called. Events that are
// registered after h.Run() has been called, won't be handled.
func (h *Handler) RegisterEventHandler(name string, fn func(Event)) {
	h.handlers[name] = fn
	h.eventNames[name] = struct{}{}
}

// Context returns the context that was passed to h.Run(). If h.Run() has not
// been called yet, nil is returned.
func (h *Handler) Context() context.Context {
	h.mux.RLock()
	defer h.mux.RUnlock()
	return h.ctx
}

// Running returns whether the handler is currently running.
func (h *Handler) Running() bool {
	h.mux.RLock()
	defer h.mux.RUnlock()
	return h.ctx != nil
}

// Run runs the handler until ctx is canceled.
func (h *Handler) Run(ctx context.Context) (<-chan error, error) {
	h.mux.Lock()
	defer h.mux.Unlock()

	if h.ctx != nil {
		return nil, ErrRunning
	}

	h.ctx = ctx

	eventNames := make([]string, 0, len(h.eventNames))
	for name := range h.eventNames {
		eventNames = append(eventNames, name)
	}

	events, errs, err := h.bus.Subscribe(ctx, eventNames...)
	if err != nil {
		return nil, fmt.Errorf("subscribe to events: %w [events=%v]", err, eventNames)
	}

	out, fail := concurrent.Errors(ctx)

	go func() {
		if err := streams.Walk(ctx, func(evt Event) error {
			if fn, ok := h.handlers[evt.Name()]; ok {
				fn(evt)
			}
			return nil
		}, events, errs); !errors.Is(err, context.Canceled) {
			fail(err)
		}
	}()

	return out, nil
}
