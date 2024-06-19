package event

import (
	"context"
	"sync"
)

// Handler manages the registration and dispatching of event handlers. It allows
// setting the mode of operation to either synchronous or asynchronous. Handlers
// can be merged, ensuring that all combined handlers operate in the same mode.
// It subscribes to an event bus to start receiving events and executing the
// appropriate handlers based on the event type.
type Handler struct {
	mux        sync.RWMutex
	handlers   map[string][]func(Event)
	async      bool
	once       sync.Once
	subscribed bool
}

// On registers a callback function for a specific event, transforming the event
// data to a specified type before invoking the callback. Returns a reference to
// the [*Handler] managing the registration.
func On[Data any](event string, fn func(Of[Data])) *Handler {
	var h Handler
	h.On(event, func(e Event) { fn(Cast[Data](e)) })
	return &h
}

// Async configures whether the event handling should be done asynchronously. It
// panics if called after the handler has been subscribed to an event source.
func (h *Handler) Async(async bool) {
	h.mux.Lock()
	defer h.mux.Unlock()
	if h.subscribed {
		panic("cannot change async mode after subscribing")
	}
	h.async = async
}

// On registers a callback function for a specific event within the handler. The
// callback will be invoked whenever the specified event is triggered.
func (h *Handler) On(event string, fn func(Event)) {
	h.once.Do(func() { h.handlers = make(map[string][]func(Event)) })

	h.mux.Lock()
	defer h.mux.Unlock()

	h.handlers[event] = append(h.handlers[event], fn)
}

// Subscribe registers the handler to start receiving events from a specified
// bus within a given context, returning a channel that streams errors
// encountered during the subscription process. If an error occurs while setting
// up the subscription, it returns immediately with the error.
func (h *Handler) Subscribe(ctx context.Context, bus Bus) (<-chan error, error) {
	eventNames := h.eventNames()

	h.mux.Lock()
	defer h.mux.Unlock()

	events, errs, err := bus.Subscribe(ctx, eventNames...)
	if err != nil {
		return nil, err
	}

	go h.callHandlers(events)

	h.subscribed = true

	return errs, nil
}

func (h *Handler) callHandlers(events <-chan Event) {
	for evt := range events {
		handlers := h.eventHandlers(evt.Name())
		for _, handler := range handlers {
			if h.async {
				go handler(evt)
				continue
			}
			handler(evt)
		}
	}
}

func (h *Handler) eventNames() []string {
	h.mux.RLock()
	defer h.mux.RUnlock()
	names := make([]string, 0, len(h.handlers))
	for name := range h.handlers {
		names = append(names, name)
	}
	return names
}

func (h *Handler) eventHandlers(event string) []func(Event) {
	h.mux.RLock()
	defer h.mux.RUnlock()
	return h.handlers[event]
}

// And combines the current [*Handler] with one or more other [*Handler]
// instances into a new, single [*Handler], enforcing that all combined handlers
// share the same mode of operation regarding asynchronous execution. If the
// modes differ, it will panic.
func (h *Handler) And(others ...*Handler) *Handler {
	async := h.isAsync()

	var merged Handler
	merged.Async(async)
	merged.merge(h)

	for _, other := range others {
		if other.isAsync() != async {
			panic("cannot merge async and non-async handlers")
		}
		merged.merge(other)
	}

	return &merged
}

func (h *Handler) isAsync() bool {
	h.mux.RLock()
	defer h.mux.RUnlock()
	return h.async
}

func (h *Handler) merge(other *Handler) {
	other.mux.RLock()
	defer other.mux.RUnlock()
	for event, handlers := range other.handlers {
		for _, handler := range handlers {
			h.On(event, handler)
		}
	}
}
