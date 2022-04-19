package aggregate

import (
	"fmt"

	"github.com/modernice/goes/event"
)

// A History provides the event history of an aggregate. A History can be
// applied to an aggregate to rebuild its current state.
type History interface {
	// Aggregate returns the reference to the aggregate of this history.
	Aggregate() Ref

	// Apply applies the history to the aggregate to rebuild its current state.
	Apply(Aggregate)
}

// ApplyHistory applies an event stream to an aggregate to reconstruct its state.
// If the aggregate implements Committer, a.RecordChange(events) and a.Commit()
// are called before returning.
func ApplyHistory[Events ~[]event.Of[any]](a Aggregate, events Events) error {
	if err := ValidateConsistency(a, events); err != nil {
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
