package aggregate

import (
	"github.com/google/uuid"
	"github.com/modernice/goes"
	"github.com/modernice/goes/event"
)

// Aggregate is an event-sourced Aggregate.
type Aggregate = AggregateOf[uuid.UUID]

// AggregateOf is an event-sourced Aggregate.
type AggregateOf[ID goes.ID] interface {
	// Aggregate returns the id, name and version of the aggregate.
	Aggregate() (ID, string, int)

	// AggregateChanges returns the uncommited events of the aggregate.
	AggregateChanges() []event.Of[any, ID]

	// ApplyEvent applies the event on the aggregate.
	ApplyEvent(event.Of[any, ID])
}

// Committer commits aggregate changes. Types that implement Committer are
// considered when applying the aggregate history onto the implementing type.
// The Commit function is called after applying the events onto the aggregate
// (using the ApplyEvent function) to commit the changes to the aggregate.
//
// *Base implements Committer.
type Committer[ID goes.ID] interface {
	// TrackChange adds events as changes to the aggregate.
	TrackChange(...event.Of[any, ID])

	// Commit commits the uncommitted changes of the aggregate. The changes
	// should be removed and the aggregate version set to the version of last
	// tracked event.
	Commit()
}
