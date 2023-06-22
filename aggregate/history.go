package aggregate

import (
	"fmt"

	"github.com/modernice/goes/event"
)

// History represents an interface for managing the application and consistency
// of events on an Aggregate. It provides methods to reference the associated
// Aggregate and apply changes to it.
type History interface {
	// Aggregate returns the Ref of the aggregate to which this History belongs. It
	// also applies the history to the given Aggregate, ensuring that events are
	// applied in a consistent manner.
	Aggregate() Ref

	// Apply applies the history of events to the given Aggregate, ensuring
	// consistency and updating the Aggregate's state accordingly. If the Aggregate
	// implements the Committer interface, changes are recorded and committed.
	Apply(Aggregate)
}

// ApplyHistory applies a sequence of events to the given Aggregate, ensuring
// consistency before applying. If the Aggregate implements the Committer
// interface, changes are recorded and committed after applying the events.
// Returns an error if consistency validation fails.
func ApplyHistory[Events ~[]event.Of[any]](a Aggregate, events Events) error {
	id, name, _ := a.Aggregate()
	version := UncommittedVersion(a)

	if err := ValidateConsistency(Ref{Name: name, ID: id}, version, events, IgnoreTime(true)); err != nil {
		return fmt.Errorf("validate consistency: %w", err)
	}

	for _, evt := range events {
		a.ApplyEvent(evt)
	}

	if c, ok := a.(Committer); ok {
		c.RecordChange(events...)
		c.Commit()
	}

	return nil
}
