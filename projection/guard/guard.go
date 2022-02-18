package guard

import (
	"log"

	"github.com/modernice/goes"
	"github.com/modernice/goes/event"
)

// Guard is a projection guard. It is used by the projection system to determine
// if an event should be applied to a projection.
type Guard[ID goes.ID] struct {
	guards map[string]func(event.Of[any, ID]) bool
}

// Option is a projection guard option.
type Option[ID goes.ID] func(*Guard[ID])

// Event returns an Option that specifies the guard for the given event. The
// projection system will call the guard before applying the given event onto a
// projection and only applies the event if the guard returns true. If the data
// of an event cannot be casted to the provided type, the event will not be
// applied.
func Event[Data any, ID goes.ID](name string, guard func(event.Of[Data, ID]) bool) Option[ID] {
	return Any(name, func(e event.Of[any, ID]) bool {
		evt, ok := event.TryCast[Data](e)
		if !ok {
			var zero Data
			log.Printf("[goes/projection/guard.Guard]: event data is not of type %T. will not apply", zero)
			return false
		}
		return guard(evt)
	})
}

// Any returns an Option that specifies the guard for the given event. The
// projection system will call the guard before applying the given event onto a
// projection and only applies the event if the guard returns true. If the data
// of an event cannot be casted to the provided type, the event will not be
// applied.
func Any[ID goes.ID](name string, guard func(event.Of[any, ID]) bool) Option[ID] {
	return func(g *Guard[ID]) {
		g.guards[name] = guard
	}
}

// New returns a new projection guard.
func New[ID goes.ID](opts ...Option[ID]) *Guard[ID] {
	g := Guard[ID]{guards: make(map[string]func(event.Of[any, ID]) bool)}
	for _, opt := range opts {
		opt(&g)
	}
	return &g
}

// GuardProjection returns true if the given event should be applied onto the projection.
func (g *Guard[ID]) GuardProjection(evt event.Of[any, ID]) bool {
	if guard, ok := g.guards[evt.Name()]; ok {
		return guard(evt)
	}
	return false
}
