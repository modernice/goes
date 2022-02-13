package projection

import "github.com/modernice/goes/event"

// Base can be embedded into projections to implement event.Handler.
type Base struct {
	appliers map[string]func(event.Event)
}

// New returns a new base for a projection. Use the RegisterHandler function to add
func New() *Base {
	return &Base{
		appliers: make(map[string]func(event.Event)),
	}
}

// ApplyWith is an alias for event.RegisterHandler.
func ApplyWith[Data any](proj event.Handler, eventName string, apply func(event.Of[Data])) {
	event.RegisterHandler(proj, eventName, apply)
}

// RegisterHandler implements event.Handler.
func (a *Base) RegisterHandler(eventName string, handler func(event.Event)) {
	a.appliers[eventName] = handler
}

// ApplyEvent implements EventApplier.
func (a *Base) ApplyEvent(evt event.Event) {
	if handler, ok := a.appliers[evt.Name()]; ok {
		handler(evt)
	}
}
