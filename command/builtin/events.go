package builtin

import (
	"github.com/modernice/goes/codec"
)

// AggregateDeleted is published when an aggregate has been deleted.
const AggregateDeleted = "goes.command.aggregate.deleted"

// AggregateDeletedData is the event data for the aggregateDeleted event.
type AggregateDeletedData struct {
	// Version is the version of the deleted aggregate.
	//
	// The aggregateVersion() returned by an AggregateDeleted event always
	// returns 0. Use this Version to see which version the aggregate has before
	// it was deleted.
	Version int
}

// RegisterEvents registers events of built-in commands into an event registry.
func RegisterEvents(r *codec.Registry) {
	gob := codec.Gob(r)
	gob.GobRegister(AggregateDeleted, func() any { return AggregateDeletedData{} })
}
