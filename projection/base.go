package projection

import "github.com/modernice/goes/event"

// Base dispatches events to handlers keyed by name. Embed it to satisfy
// [Target] with minimal effort.
type Base struct {
	appliers map[string]func(event.Event)
}

// New returns an empty Base.
func New() *Base {
	return &Base{
		appliers: make(map[string]func(event.Event)),
	}
}

// RegisterEventHandler associates handler with an event name.
func (a *Base) RegisterEventHandler(eventName string, handler func(event.Event)) {
	a.appliers[eventName] = handler
}

// RegisteredEvents lists all event names with handlers.
func (a *Base) RegisteredEvents() []string {
	events := make([]string, 0, len(a.appliers))
	for evt := range a.appliers {
		events = append(events, evt)
	}
	return events
}

// ApplyEvent dispatches evt if a handler is registered for its name.
func (a *Base) ApplyEvent(evt event.Event) {
	if handler, ok := a.appliers[evt.Name()]; ok {
		handler(evt)
	}
}
