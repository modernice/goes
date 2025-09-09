package aggregate

import (
	"fmt"

	"github.com/modernice/goes/event"
)

// History is a sequence of events for an aggregate.
type History interface {
	// Aggregate returns the referenced aggregate.
	Aggregate() Ref

	// Apply replays the events on a.
	Apply(Aggregate)
}

// ApplyHistory replays events on a after validating their consistency. If a
// implements [Committer] the events are recorded and committed.
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
