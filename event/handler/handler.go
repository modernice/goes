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

// ErrRunning is an error that indicates an event handler is already running. It
// is returned when attempting to run a handler that is currently active. This
// error serves as a guard against concurrent execution of the same event
// handler.
var ErrRunning = errors.New("event handler is already running")

// Handler is a type that processes events from an event bus. It associates
// event names with specific functions, which are called whenever their
// respective event occurs. Handler uses multiple workers to process events
// concurrently. The number of workers can be customized through options when
// creating a new Handler instance. Events can be registered to the Handler, and
// it provides methods to check if it's currently processing events or if a
// certain event has a registered handler. Handlers also have a context which
// can be used for synchronization and cancellation of operations. Handlers
// prevent concurrent execution of the same instance to avoid race conditions.
type Handler struct {
	bus          event.Bus
	startupStore event.Store
	workers      int

	mux        sync.RWMutex
	handlers   map[string]func(event.Event)
	eventNames map[string]struct{}
	ctx        context.Context
}

// Option is a function type used to configure a [Handler]. It can be used to
// set various properties of the [Handler] such as the event store or the number
// of workers. Each option is applied in the order they are provided when
// constructing a new [Handler] using the New function.
type Option func(*Handler)

// Startup sets the startup event store for a [Handler]. This store is used to
// handle events when the [Handler] starts up. The Startup option is typically
// used to initialize the system with initial event handling on startup or
// implement a "catch-up" mechanism for their event handlers.
func Startup(store event.Store) Option {
	return func(h *Handler) {
		h.startupStore = store
	}
}

// WithStore is an [Option] for a [Handler] that sets the event store to be
// used. This function returns an [Option] that, when used with the New
// function, configures a [Handler] to use the specified event store. This is
// typically used to specify where events should be stored when they are handled
// by the [Handler]. Note that WithStore is equivalent to the Startup function.
//
// Deprecated: Use Startup instead.
func WithStore(store event.Store) Option {
	return Startup(store)
}

// Workers sets the number of workers that a [Handler] uses to process events.
// If this option is not used when constructing a new [Handler], the default
// number of workers is 1.
func Workers(n int) Option {
	return func(h *Handler) {
		h.workers = n
	}
}

// New creates a new event handler with the provided bus and options. It sets up
// an empty map for handlers and event names, applies the given options, and
// ensures that there is at least one worker. The new handler is returned.
func New(bus event.Bus, opts ...Option) *Handler {
	h := &Handler{
		bus:        bus,
		handlers:   make(map[string]func(event.Event)),
		eventNames: make(map[string]struct{}),
	}
	for _, opt := range opts {
		opt(h)
	}
	if h.workers < 1 {
		h.workers = 1
	}
	return h
}

// RegisterEventHandler associates the provided event name with a given function
// to handle that event. The function will be called whenever an event with the
// associated name occurs. This method is safe for concurrent use.
func (h *Handler) RegisterEventHandler(name string, fn func(event.Event)) {
	h.mux.Lock()
	defer h.mux.Unlock()
	h.handlers[name] = fn
	h.eventNames[name] = struct{}{}
}

// EventHandler retrieves the event handler function associated with the
// provided event name. If a handler for the given event name is found, it
// returns the handler function and true. If no handler is found, it returns nil
// and false. This method is safe for concurrent use.
func (h *Handler) EventHandler(name string) (func(event.Event), bool) {
	h.mux.RLock()
	defer h.mux.RUnlock()
	fn, ok := h.handlers[name]
	return fn, ok
}

// Context returns the context of the Handler. This context is used for
// synchronization and cancellation of operations within the Handler. If the
// Handler is not running, nil is returned.
func (h *Handler) Context() context.Context {
	h.mux.RLock()
	defer h.mux.RUnlock()
	return h.ctx
}

// Running checks if the Handler is currently processing events. It returns true
// if the Handler is running and false otherwise. This method is safe for
// concurrent use.
func (h *Handler) Running() bool {
	h.mux.RLock()
	defer h.mux.RUnlock()
	return h.ctx != nil
}

// Run starts the event handling process for the [Handler]. It subscribes to the
// events that have been registered with the [Handler] and starts processing
// them concurrently with a number of worker goroutines. The errors from running
// the handlers are returned through a channel. If the [Handler] is already
// running when this method is called, it returns an error. The context passed
// to Run is used to control cancellation of the event handling process.
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

	queueError, fail := concurrent.Errors(ctx)
	queue := make(chan event.Event)

	go func() {
		defer close(queue)
		if err := streams.Walk(ctx, func(evt event.Event) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case queue <- evt:
				return nil
			}
		}, events, errs); !errors.Is(err, context.Canceled) {
			fail(err)
		}
	}()

	out := streams.FanInAll(queueError, h.handleEvents(ctx, queue))

	if h.startupStore != nil {
		go func() {
			if err := h.startup(ctx, eventNames); err != nil {
				fail(fmt.Errorf("startup handler: %w", err))
			}
		}()
	}

	return out, nil
}

func (h *Handler) handleEvents(ctx context.Context, events <-chan event.Event) <-chan error {
	errs, fail := concurrent.Errors(ctx)
	for i := 0; i < h.workers; i++ {
		go func() {
			for evt := range events {
				fn, ok := h.EventHandler(evt.Name())
				if !ok {
					fail(fmt.Errorf("no handler for event %q", evt.Name()))
					continue
				}
				fn(evt)
			}
		}()
	}
	return errs
}

func (h *Handler) startup(ctx context.Context, eventNames []string) error {
	str, errs, err := h.startupStore.Query(ctx, query.New(
		query.Name(eventNames...),
		query.SortByTime(),
	))
	if err != nil {
		return fmt.Errorf("query events %v: %w", eventNames, err)
	}

	handlerErrors := h.handleEvents(ctx, str)

	for err := range streams.FanInAll(errs, handlerErrors) {
		return err
	}

	return nil
}
