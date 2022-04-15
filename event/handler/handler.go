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

// ErrRunning is returned when trying to run a *Handler that is already running.
var ErrRunning = errors.New("event handler is already running")

// A Handler asynchronously handles published events.
//
//	var bus event.Bus
//	h := handler.New(bus)
//	event.HandleWith(h, func(evt event.Of[FooData]) {...}, "foo")
//	event.HandleWith(h, func(evt event.Of[BarData]) {...}, "bar")
//
//	errs, err := h.Run(context.TODO())
type Handler struct {
	bus        event.Bus
	store      event.Store // optional
	handlers   map[string]func(event.Event)
	eventNames map[string]struct{}

	mux sync.RWMutex
	ctx context.Context
}

// Option is a handler option.
type Option func(*Handler)

// WithStore returns an Option that makes a Handler query the registered events
// from the provided store on startup and handle them as if they were published
// over the underlying event bus. This allows to handle "past" events on startup.
func WithStore(store event.Store) Option {
	return func(h *Handler) {
		h.store = store
	}
}

// New returns an event handler for published events.
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

// RegisterEventHandler registers the handler for the given event.
// Events must be registered before h.Run() is called. events that are
// registered after h.Run() has been called, won't be handled.
func (h *Handler) RegisterEventHandler(name string, fn func(event.Event)) {
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
		if err := streams.Walk(ctx, func(evt event.Event) error {
			if fn, ok := h.handlers[evt.Name()]; ok {
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
		if fn, ok := h.handlers[evt.Name()]; ok {
			fn(evt)
		}
		return nil
	}, str, errs)
}
