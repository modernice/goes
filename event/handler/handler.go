package handler

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/query"
	"github.com/modernice/goes/helper/streams"
	"github.com/modernice/goes/internal/concurrent"
)

// ErrRunning is the error returned when trying to run an event handler that is
// already running.
var ErrRunning = errors.New("event handler is already running")

// Handler is responsible for managing the registration and execution of event
// handlers in response to incoming events. It maintains a map of registered
// event handlers and listens to an event bus for events that match registered
// handler names. When such an event occurs, the corresponding handler function
// is invoked. Handler also provides support for querying stored events from an
// event store, and it can run concurrently, with its execution state being
// controlled through a provided context.
type Handler struct {
	bus   event.Bus
	store event.Store

	mux        sync.RWMutex
	handlers   map[string]func(event.Event)
	eventNames map[string]struct{}
	ctx        context.Context
}

// Option is a function that modifies the configuration of a [Handler]. It
// provides a way to set optional parameters when creating a new [Handler]. This
// approach is used to avoid constructor cluttering when there are many
// configurations possible for a [Handler].
type Option func(*Handler)

// WithStore is a function that returns an Option which, when applied to a
// Handler, sets the event.Store of that Handler. This is used to configure
// where the Handler stores events.
func WithStore(store event.Store) Option {
	return func(h *Handler) {
		h.store = store
	}
}

// New creates a new Handler with the provided event bus and applies the given
// options. The bus parameter is used to subscribe to events and distribute them
// to registered event handlers. Options can be used to configure the Handler,
// for example to specify an event store where past events are queried from on
// startup. The returned Handler is not running and must be started with the Run
// method.
func New(bus event.Bus, opts ...Option) *Handler {
	h := &Handler{
		bus:        bus,
		handlers:   make(map[string]func(event.Event)),
		eventNames: make(map[string]struct{}),
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// RegisterEventHandler registers an event handler function for the given event
// name. The event handler function, which takes an [event.Event] as a
// parameter, will be called whenever an event with the registered name occurs.
// The function is thread-safe and can be called concurrently.
func (h *Handler) RegisterEventHandler(name string, fn func(event.Event)) {
	h.mux.Lock()
	defer h.mux.Unlock()
	h.handlers[name] = fn
	h.eventNames[name] = struct{}{}
}

// EventHandler retrieves the event handling function associated with the
// provided name. The function returned is responsible for processing an event
// of that name. If no handler is found for the given name, a nil function and
// false are returned.
func (h *Handler) EventHandler(name string) (func(event.Event), bool) {
	h.mux.RLock()
	defer h.mux.RUnlock()
	fn, ok := h.handlers[name]
	return fn, ok
}

// Context returns the context of the Handler. This context is set when the Run
// method is called and is used to control the lifecycle of the Handler. It's
// protected by a read-write lock, ensuring safe concurrent access.
func (h *Handler) Context() context.Context {
	h.mux.RLock()
	defer h.mux.RUnlock()
	return h.ctx
}

// Running checks if the Handler is currently running. It returns true if the
// Handler has been started and not yet stopped, otherwise it returns false. The
// context of the Handler is used to determine its state. If the context is not
// nil, it indicates that the Handler is running.
func (h *Handler) Running() bool {
	h.mux.RLock()
	defer h.mux.RUnlock()
	return h.ctx != nil
}

// Run starts the event handler with the provided context. It subscribes to all
// registered events on the event bus and begins handling incoming events. If
// the event handler has an event store, it also handles all stored events. If
// the handler is already running, it returns an error. The function returns a
// channel for errors that may occur during event handling and an error if there
// was an issue starting the handler.
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
		if err := streams.Walk(ctx, func(evt event.Event) error {
			if fn, ok := h.EventHandler(evt.Name()); ok {
				fn(evt)
			}
			return nil
		}, events, errs); !errors.Is(err, context.Canceled) {
			fail(err)
		}
	}()

	if h.store != nil {
		go func() {
			if err := h.handleStoredEvents(ctx, eventNames); err != nil {
				fail(fmt.Errorf("handle events from store: %w", err))
			}
		}()
	}

	return out, nil
}

func (h *Handler) handleStoredEvents(ctx context.Context, eventNames []string) error {
	str, errs, err := h.store.Query(ctx, query.New(
		query.Name(eventNames...),
		query.SortByTime(),
	))
	if err != nil {
		return fmt.Errorf("query %s events: %w", eventNames, err)
	}

	return streams.Walk(ctx, func(evt event.Event) error {
		if fn, ok := h.EventHandler(evt.Name()); ok {
			fn(evt)
		}
		return nil
	}, str, errs)
}
