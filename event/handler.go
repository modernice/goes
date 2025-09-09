package event

import (
	"context"
	"sync"
)

// Handler routes events to registered callbacks. It can execute handlers
// synchronously or asynchronously and can be merged with other Handlers.
type Handler struct {
	mux        sync.RWMutex
	handlers   map[string][]func(Event)
	async      bool
	once       sync.Once
	subscribed bool
}

// On registers a typed callback for event and returns the new Handler.
func On[Data any](event string, fn func(Of[Data])) *Handler {
	var h Handler
	h.On(event, func(e Event) { fn(Cast[Data](e)) })
	return &h
}

// Async toggles asynchronous handler execution. It must be called before
// Subscribe.
func (h *Handler) Async(async bool) *Handler {
	h.mux.Lock()
	defer h.mux.Unlock()
	if h.subscribed {
		panic("cannot change async mode after subscribing")
	}
	h.async = async
	return h
}

// On registers fn for events named event.
func (h *Handler) On(event string, fn func(Event)) {
	h.once.Do(func() { h.handlers = make(map[string][]func(Event)) })

	h.mux.Lock()
	defer h.mux.Unlock()

	h.handlers[event] = append(h.handlers[event], fn)
}

// Subscribe begins receiving events from bus. It returns a channel of handler
// errors.
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

// And merges h with others and returns a new Handler containing all handlers.
func (h *Handler) And(others ...*Handler) *Handler {
	async := h.isAsync()

	var merged Handler
	merged.Async(async)
	merged.merge(h)

	for _, other := range others {
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
