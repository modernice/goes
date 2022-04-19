package aggregate

import (
	"github.com/google/uuid"
	"github.com/modernice/goes/event"
)

// Aggregate is an event-sourced aggregate.
type Aggregate = Of[uuid.UUID]

// Of is an event-sourced aggregate.
type Of[ID comparable] interface {
	// Aggregate returns the id, name and version of the aggregate.
	Aggregate() (ID, string, int)

	// AggregateChanges returns the uncommited events of the aggregate.
	AggregateChanges() []event.Event

	// ApplyEvent applies an event to the aggregate.
	ApplyEvent(event.Event)
}

// A Committer is an aggregate that records and commits its changes.
// The ApplyHistory() function calls RecordChange() and Commit() to reconstruct
// the state of an aggregate, and the aggregate repository calls Commit() after
// saving the aggregate changes to the event store.
type Committer interface {
	// RecordChange records events that were applied to the aggregate.
	RecordChange(...event.Event)

	// Commit clears the recorded changes and updates the current version of the
	// aggregate to the last recorded event.
	Commit()
}

// SoftDeleter is an API that can be implemented by event data to soft-delete an
// aggregate. Soft-deleted aggregates are excluded from query results of the
// aggregate repository. When trying to fetch a soft-deleted aggregate from a
// repository, a repository.ErrDeleted error is returned. Soft-deleted
// aggregates can be restored by SoftRestorer events.
//
// To soft-delete an aggregate, an event with event data that implements
// SoftDeleter must be inserted into the aggregate's event stream.
// The SoftDelete() method of the event data must return true if the aggregate
// should be soft-deleted.
//
//	type DeletedData struct {}
//	func (DeletedData) SoftDelete() bool { return true }
//
//	type Foo struct { *aggregate.Base }
//	func (f *Foo) Delete() {
//		aggregate.Next(f, "foo.deleted", DeletedData{})
//	}
type SoftDeleter interface{ SoftDelete() bool }

// SoftRestorer is an API that can be implemented by event data to restore a
// soft-deleted aggregate.
//
// To restore a soft-deleted aggregate, an event with event data that implements
// SoftRestorer must be inserted into the aggregate's event stream.
// The SoftDelete() method of the event data must return true if the aggregate
// should be restored.
//
//	type RestoredData struct {}
//	func (RestoredData) SoftRestore() bool { return true }
//
//	type Foo struct { *aggregate.Base }
//	func (f *Foo) Delete() {
//		aggregate.Next(f, "foo.restored", RestoredData{})
//	}
type SoftRestorer interface{ SoftRestore() bool }

// Ref is a reference to a specific aggregate, identified by its name and id.
type Ref = event.AggregateRef
