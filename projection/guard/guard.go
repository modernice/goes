package guard

import (
	"log"

	"github.com/modernice/goes/event"
)

// Guard holds per-event predicates used to filter events for a projection.
type Guard struct {
	guards map[string]func(event.Event) bool
}

// Option configures a [Guard].
type Option func(*Guard)

// Event registers a typed guard for an event name. The guard is only executed if
// the event's data matches Data; mismatches are ignored.
func Event[Data any](name string, guard func(event.Of[Data]) bool) Option {
	return Any(name, func(e event.Event) bool {
		evt, ok := event.TryCast[Data](e)
		if !ok {
			var zero Data
			log.Printf("[goes/projection/guard.Guard]: event data is not of type %T. will not apply", zero)
			return false
		}
		return guard(evt)
	})
}

// Any registers a guard for the named event.
func Any(name string, guard func(event.Event) bool) Option {
	return func(g *Guard) {
		g.guards[name] = guard
	}
}

// New creates a Guard.
func New(opts ...Option) *Guard {
	g := Guard{guards: make(map[string]func(event.Event) bool)}
	for _, opt := range opts {
		opt(&g)
	}
	return &g
}

// GuardProjection reports whether evt passes a registered guard.
func (g *Guard) GuardProjection(evt event.Event) bool {
	if guard, ok := g.guards[evt.Name()]; ok {
		return guard(evt)
	}
	return false
}
