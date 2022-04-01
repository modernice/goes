package aggregate

import (
	"github.com/google/uuid"
	"github.com/modernice/goes/event"
)

// Aggregate is an event-sourced aggregate.
type Aggregate = AggregateOf[uuid.UUID]

// AggregateOf is an event-sourced aggregate.
type AggregateOf[ID comparable] interface {
	// Aggregate returns the id, name and version of the aggregate.
	Aggregate() (ID, string, int)

	// AggregateChanges returns the uncommited events of the aggregate.
	AggregateChanges() []event.Event

	// ApplyEvent applies the event on the aggregate.
	ApplyEvent(event.Event)
}

// Committer commits aggregate changes. Types that implement Committer are
// considered when applying the aggregate history onto the implementing type.
// The Commit function is called after applying the events onto the aggregate
// (using the ApplyEvent function) to commit the changes to the aggregate.
//
// *Base implements Committer.
type Committer interface {
	// TrackChange adds events as changes to the aggregate.
	TrackChange(...event.Event)

	// Commit commits the uncommitted changes of the aggregate. The changes
	// should be removed and the aggregate version set to the version of last
	// tracked event.
	Commit()
}

// SoftDeleter can be implemented by aggregate events to soft-delete an
// aggregate. A soft-deletes aggregate is excluded from query results of the
// aggregate repository. When trying to fetch a soft-deleted aggregate through
// the aggregate repository, a repository.ErrDeleted error will be returned.
// Soft-deleted aggregates may be restored by a SoftRestorer event.
type SoftDeleter interface{ SoftDelete() bool }

// SoftRestorer can be implemented by aggregate eevntw to restore a soft-deleted
// aggregate (see SoftDeleter).
type SoftRestorer interface{ SoftRestore() bool }

// Ref is a reference to a specific aggregate, identified by its name and id.
type Ref = event.AggregateRef
