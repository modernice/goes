package aggregate

import (
	"github.com/google/uuid"
	"github.com/modernice/goes/event"
)

// Aggregate is an event-sourced entity. It reports its ID, name and version,
// lists uncommitted events and applies new ones. Implementations may also
// satisfy [Committer] and the soft deletion helpers below.
type Aggregate = Of[uuid.UUID]

// Of describes an aggregate with a comparable identifier. It returns ID, name
// and version, exposes uncommitted events and applies incoming events. Further
// APIs like [Committer], [SoftDeleter] or [SoftRestorer] can be built on top.
type Of[ID comparable] interface {
	// Aggregate returns ID, name and version.
	Aggregate() (ID, string, int)

	// AggregateChanges lists uncommitted events.
	AggregateChanges() []event.Event

	// ApplyEvent applies an event to the aggregate.
	ApplyEvent(event.Event)
}

// Committer records and flushes changes to an aggregate.
type Committer interface {
	// RecordChange appends uncommitted events.
	RecordChange(...event.Event)

	// Commit finalizes recorded changes.
	Commit()
}

// SoftDeleter marks an event as soft-deleting its aggregate. Repositories skip
// aggregates whose streams contain such a marker until a [SoftRestorer] event
// appears.
//
//	type Deleted struct{}
//	func (Deleted) SoftDelete() bool { return true }
type SoftDeleter interface{ SoftDelete() bool }

// SoftRestorer marks an event as restoring a previously soft-deleted
// aggregate.
//
//	type Restored struct{}
//	func (Restored) SoftRestore() bool { return true }
type SoftRestorer interface{ SoftRestore() bool }

// Ref references an aggregate by name and id.
type Ref = event.AggregateRef
