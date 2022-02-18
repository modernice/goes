package projection

import (
	"github.com/modernice/goes"
	"github.com/modernice/goes/event"
)

// Base can be embedded into projections to implement event.Handler.
type Base[ID goes.ID] struct {
	appliers map[string]func(event.Of[any, ID])
}

// New returns a new base for a projection. Use the RegisterHandler function to add
func New[ID goes.ID]() *Base[ID] {
	return &Base[ID]{
		appliers: make(map[string]func(event.Of[any, ID])),
	}
}

// ApplyWith is an alias for event.RegisterHandler.
func ApplyWith[Data any, ID goes.ID, Event event.Of[Data, ID]](proj event.Handler[ID], eventName string, apply func(Event)) {
	event.RegisterHandler[Data](proj, eventName, apply)
}

// RegisterHandler implements event.Handler.
func (a *Base[ID]) RegisterHandler(eventName string, handler func(event.Of[any, ID])) {
	a.appliers[eventName] = handler
}

// ApplyEvent implements EventApplier.
func (a *Base[ID]) ApplyEvent(evt event.Of[any, ID]) {
	if handler, ok := a.appliers[evt.Name()]; ok {
		handler(evt)
	}
}
