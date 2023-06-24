package projection

import "github.com/modernice/goes/event"

// Base is a simple implementation of a projection that allows registering event
// handlers and applying events to registered handlers. Handlers can be
// registered for specific event names using the RegisterEventHandler method,
// and events can be applied using the ApplyEvent method. RegisteredEvents
// returns a slice of event names that have registered handlers in the Base
// projection.
type Base struct {
	appliers map[string]func(event.Event)
}

// New creates and returns a new Base projection with an empty appliers map for
// registering event handlers.
func New() *Base {
	return &Base{
		appliers: make(map[string]func(event.Event)),
	}
}

// RegisterEventHandler associates the given event handler function with the
// specified event name in the Base projection. The handler will be called when
// an event with the matching name is applied to the projection using ApplyEvent
// method.
func (a *Base) RegisterEventHandler(eventName string, handler func(event.Event)) {
	a.appliers[eventName] = handler
}

// RegisteredEvents returns a slice of event names that have registered handlers
// in the projection.
func (a *Base) RegisteredEvents() []string {
	events := make([]string, 0, len(a.appliers))
	for evt := range a.appliers {
		events = append(events, evt)
	}
	return events
}

// ApplyEvent applies the given event to the Base projection by calling its
// registered event handler, if one exists for the event name. If no handler is
// registered for the event name, ApplyEvent does nothing.
func (a *Base) ApplyEvent(evt event.Event) {
	if handler, ok := a.appliers[evt.Name()]; ok {
		handler(evt)
	}
}
