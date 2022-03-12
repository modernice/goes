package auth

import (
	"context"

	"github.com/google/uuid"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/projection/lookup"
	"github.com/modernice/goes/projection/schedule"
)

const (
	// LookupActor lookups the aggregate id of an actor from a given actor id.
	LookupActor = "actor"
)

// Lookup provides lookups from actor ids to aggregate ids of those actors.
type Lookup struct {
	*lookup.Lookup
}

var lookupEvents = [...]string{ActorIdentified}

// NewLookup returns a new lookup for aggregate ids of actors.
func NewLookup(store event.Store, bus event.Bus, opts ...schedule.ContinuousOption) *Lookup {
	return &Lookup{Lookup: lookup.New(store, bus, lookupEvents[:], opts...)}
}

// Actor returns the aggregate id of the actor with the given formatted actor id.
func (l *Lookup) Actor(ctx context.Context, id string) (uuid.UUID, bool) {
	return l.Reverse(ctx, ActorAggregate, LookupActor, id)
}

// ProvideLookup implements lookup.Event.
func (data ActorIdentifiedData) ProvideLookup(p lookup.Provider) {
	p.Provide(LookupActor, string(data))
}
