package consistency

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/modernice/goes/event"
)

const (
	// UnknownKind is the invalid Kind.
	UnknownKind = Kind(iota)

	// ID means there is an inconsistency in the ID of an Aggregate.
	ID

	// Name means there is an inconsistency in the Aggregate names of the Events
	// of an Aggregate.
	Name

	// Version means there is an inconsistency in the Event versions of an
	// Aggregate.
	Version

	// Time means there is an inconsistency in the Event times of an Aggregate.
	Time
)

// Error is a consistency error.
type Error struct {
	// Kind is the kind of consistency error.
	Kind Kind
	// Aggregate is the handled aggregate.
	Aggregate Aggregate
	// Events are the tested events.
	Events []event.Event
	// EventIndex is the index of the Event that caused the Error.
	EventIndex int
}

// Kind is the kind of inconsistency.
type Kind int

// Aggregate is a subset of aggregate.Aggregate. Redeclared here to avoid
// import cycles.
type Aggregate interface {
	// AggregateID returns the UUID of the Aggregate.
	AggregateID() uuid.UUID
	// AggregateName returns the name of the Aggregate.
	AggregateName() string
	// AggregateVersion returns the version of the Aggregate.
	AggregateVersion() int
	// AggregateChanges returns the changes of the Aggregate.
	AggregateChanges() []event.Event
}

// Validate tests the consistency of the given Events against the Aggregate a.
//
// An Event e is invalid if e.AggregateName() doesn't match a.AggregateName(),
// e.AggregateID() doesn't match a.AggregateID() or if e.AggregateVersion()
// doesn't match the position in events relative to a.AggregateVersion(). This
// means that events[0].AggregateVersion() must equal a.AggregateVersion() + 1,
// events[1].AggregateVersion() must equal a.AggregateVersion() + 2 etc.
//
// An Event a is also invalid if its time is equal to or after the time of the
// previous Event.
//
// The first Event e in events that is invalid causes Validate to return an
// *Error containing the Kind of inconsistency and the Event that caused the
// inconsistency.
func Validate(a Aggregate, events ...event.Event) error {
	id := a.AggregateID()
	name := a.AggregateName()
	version := currentVersion(a)
	cv := version
	var prev event.Event
	for i, evt := range events {
		if evt.AggregateID() != id {
			return &Error{
				Kind:       ID,
				Aggregate:  a,
				Events:     events,
				EventIndex: i,
			}
		}
		if evt.AggregateName() != name {
			return &Error{
				Kind:       Name,
				Aggregate:  a,
				Events:     events,
				EventIndex: i,
			}
		}
		if evt.AggregateVersion() != cv+1 {
			return &Error{
				Kind:       Version,
				Aggregate:  a,
				Events:     events,
				EventIndex: i,
			}
		}
		if prev != nil && (prev.Time().Equal(evt.Time()) || prev.Time().After(evt.Time())) {
			return &Error{
				Kind:       Time,
				Aggregate:  a,
				Events:     events,
				EventIndex: i,
			}
		}
		prev = evt
		cv++
	}
	return nil
}

// Event return the first Event that caused an inconsistency.
func (err *Error) Event() event.Event {
	if err.EventIndex < 0 || err.EventIndex >= len(err.Events) {
		return nil
	}
	return err.Events[err.EventIndex]
}

func (err *Error) Error() string {
	evt := err.Event()
	switch err.Kind {
	case ID:
		return fmt.Sprintf(
			"consistency: %q event has invalid AggregateID. want=%s got=%s",
			evt.Name(), err.Aggregate.AggregateID(), evt.AggregateID(),
		)
	case Name:
		return fmt.Sprintf(
			"consistency: %q event has invalid AggregateName. want=%s got=%s",
			evt.Name(), err.Aggregate.AggregateName(), evt.AggregateName(),
		)
	case Version:
		return fmt.Sprintf(
			"consistency: %q event has invalid AggregateVersion. want=%d got=%d",
			evt.Name(), currentVersion(err.Aggregate)+1+err.EventIndex, evt.AggregateVersion(),
		)
	default:
		return fmt.Sprintf("consistency: invalid inconsistency kind=%d", err.Kind)
	}
}

func (k Kind) String() string {
	switch k {
	case ID:
		return "Kind(ID)"
	case Name:
		return "Kind(Name)"
	case Version:
		return "Kind(Version)"
	default:
		return "Kind(Unknown)"
	}
}

func currentVersion(a Aggregate) int {
	return a.AggregateVersion() + len(a.AggregateChanges())
}
