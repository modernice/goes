package aggregate

import (
	"github.com/google/uuid"
	"github.com/modernice/goes/event"
)

// Aggregate is an event-sourced Aggregate.
type Aggregate interface {
	// Aggregate returns the id, name and version of the aggregate.
	Aggregate() (uuid.UUID, string, int)

	// AggregateChanges returns the uncommited events of the aggregate.
	AggregateChanges() []event.Event[any]

	// ApplyEvent applies the event on the aggregate.
	ApplyEvent(event.Event[any])
}

// Committer commits aggregate changes. Types that implement Committer are
// considered when applying the aggregate history onto the implementing type.
// The Commit function is called after applying the events onto the aggregate
// (using the ApplyEvent function) to commit the changes to the aggregate.
//
// *Base implements Committer.
type Committer interface {
	// TrackChange adds events as changes to the aggregate.
	TrackChange(...event.Event[any])

	// Commit commits the uncommitted changes of the aggregate. The changes
	// should be removed and the aggregate version set to the version of last
	// tracked event.
	Commit()
}
