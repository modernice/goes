package aggregate

import (
	"github.com/google/uuid"
	"github.com/modernice/goes/event"
)

// Aggregate represents an entity that encapsulates a collection of events and
// their resulting state. It provides methods to apply new events, access the
// current state, and retrieve the list of uncommitted changes. It can also be
// used in conjunction with [Committer] to record and commit changes.
// Additionally, Aggregate supports soft deletion and restoration through the
// [SoftDeleter] and [SoftRestorer] interfaces.
type Aggregate = Of[uuid.UUID]

// Of is an interface that represents an event-sourced aggregate with a
// comparable ID. It provides methods to access the aggregate's ID, name,
// version, and uncommitted changes, as well as to apply an event to the
// aggregate. Implementations of this interface can be used alongside other
// types such as [Committer], [SoftDeleter], and [SoftRestorer] to facilitate
// event sourcing and soft deletion/restoration of aggregates.
type Of[ID comparable] interface {
	// Aggregate returns the ID, name, and version of an [Of] aggregate. It also
	// provides methods for obtaining the aggregate's changes as a slice of
	// [event.Event] and applying an event to the aggregate.
	Aggregate() (ID, string, int)

	// AggregateChanges returns a slice of all uncommitted events that have been
	// recorded for the aggregate.
	AggregateChanges() []event.Event

	// ApplyEvent applies the given event.Event to the aggregate, updating its state
	// according to the event data.
	ApplyEvent(event.Event)
}

// Committer is an interface that handles recording and committing changes in
// the form of events to an aggregate. It provides methods for recording changes
// as a series of events and committing them to update the aggregate's state.
type Committer interface {
	// RecordChange records the given events and associates them with the aggregate
	// in the Committer. These events will be applied and persisted when Commit is
	// called.
	RecordChange(...event.Event)

	// Commit records the changes made to an aggregate by appending the recorded
	// events to the aggregate's event stream.
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
