package aggregate

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/modernice/goes/event"
)

const (
	// ID means there is an inconsistency in the aggregate ids.
	InconsistentID = ConsistencyKind(iota + 1)

	// Name means there is an inconsistency in the aggregate names.
	InconsistentName

	// Version means there is an inconsistency in the event versions.
	InconsistentVersion

	// Time means there is an inconsistency in the event times.
	InconsistentTime
)

// Error is a consistency error.
type ConsistencyError struct {
	// Kind is the kind of incosistency.
	Kind ConsistencyKind
	// Aggregate is the handled aggregate.
	Aggregate Ref
	// CurrentVersion is the current version of the aggregate.
	CurrentVersion int
	// Events are the tested events.
	Events []event.Event
	// EventIndex is the index of the event that caused the Error.
	EventIndex int
}

// ConsistencyKind is the kind of inconsistency.
type ConsistencyKind int

// IsConsistencyError returns whether an error was caused by an inconsistency in
// the events of an aggregate. An error is considered a consistency error if it
// either unwraps to a *ConsistencyError or if it has an IsConsistencyError() bool
// method that return true for the given error.
func IsConsistencyError(err error) bool {
	var cerr *ConsistencyError
	if errors.As(err, &cerr) {
		return true
	}

	var ierr interface{ IsConsistencyError() bool }
	if errors.As(err, &ierr) {
		return ierr.IsConsistencyError()
	}

	return false
}

// Validate tests the consistency of the given events against the given aggregate.
//
// An event e is invalid if e.AggregateName() doesn't match a.AggregateName(),
// e.AggregateID() doesn't match a.AggregateID() or if e.AggregateVersion()
// doesn't match the position in events relative to a.AggregateVersion(). This
// means that events[0].AggregateVersion() must equal a.AggregateVersion() + 1,
// events[1].AggregateVersion() must equal a.AggregateVersion() + 2 etc.
//
// An event a is also invalid if its time is equal to or after the time of the
// previous event.
//
// The first event e in events that is invalid causes Validate to return an
// *Error containing the Kind of inconsistency and the event that caused the
// inconsistency.
func ValidateConsistency[Data any, Events ~[]event.Of[Data]](a Aggregate, events Events) error {
	id, name, _ := a.Aggregate()
	ref := Ref{
		Name: name,
		ID:   id,
	}

	aevents := make([]event.Event, len(events))
	for i, evt := range events {
		aevents[i] = event.Any(evt)
	}

	var hasPrevEvent bool
	var prevEvent event.Event
	var prevVersion int

	cv := currentVersion(a)

	for i, evt := range aevents {
		eid, ename, ev := evt.Aggregate()
		if eid != id {
			return &ConsistencyError{
				Kind:           InconsistentID,
				Aggregate:      ref,
				CurrentVersion: cv,
				Events:         aevents,
				EventIndex:     i,
			}
		}
		if ename != name {
			return &ConsistencyError{
				Kind:           InconsistentName,
				Aggregate:      ref,
				CurrentVersion: cv,
				Events:         aevents,
				EventIndex:     i,
			}
		}
		if ev <= 0 {
			return &ConsistencyError{
				Kind:           InconsistentVersion,
				Aggregate:      ref,
				CurrentVersion: cv,
				Events:         aevents,
				EventIndex:     i,
			}
		}
		if ev <= cv {
			return &ConsistencyError{
				Kind:           InconsistentVersion,
				Aggregate:      ref,
				CurrentVersion: cv,
				Events:         aevents,
				EventIndex:     i,
			}
		}
		if hasPrevEvent && ev <= prevVersion {
			return &ConsistencyError{
				Kind:           InconsistentVersion,
				Aggregate:      ref,
				CurrentVersion: cv,
				Events:         aevents,
				EventIndex:     i,
			}
		}
		if hasPrevEvent {
			nano := evt.Time().UnixNano()
			prevNano := prevEvent.Time().UnixNano()
			if nano <= prevNano {
				return &ConsistencyError{
					Kind:           InconsistentTime,
					Aggregate:      ref,
					CurrentVersion: cv,
					Events:         aevents,
					EventIndex:     i,
				}
			}
		}
		prevEvent = evt
		prevVersion = ev
		hasPrevEvent = true
	}
	return nil
}

// Event return the first event that caused an inconsistency.
func (err *ConsistencyError) Event() event.Event {
	if err.EventIndex < 0 || err.EventIndex >= len(err.Events) {
		return nil
	}
	return err.Events[err.EventIndex]
}

func (err *ConsistencyError) Error() string {
	evt := err.Event()
	var (
		id   uuid.UUID
		name string
		v    int
	)
	if evt != nil {
		id, name, v = evt.Aggregate()
	}

	var (
		aid   uuid.UUID
		aname string
	)

	aid, aname, _ = err.Aggregate.Aggregate()

	switch err.Kind {
	case InconsistentID:
		return fmt.Sprintf(
			"consistency: %q event has invalid AggregateID. want=%s got=%s",
			evt.Name(), aid, id,
		)
	case InconsistentName:
		return fmt.Sprintf(
			"consistency: %q event has invalid AggregateName. want=%s got=%s",
			evt.Name(), aname, name,
		)
	case InconsistentVersion:
		return fmt.Sprintf(
			"consistency: %q event has invalid AggregateVersion. want=%d got=%d",
			evt.Name(), err.CurrentVersion+1+err.EventIndex, v,
		)
	case InconsistentTime:
		return fmt.Sprintf(
			"consistency: %q event has invalid Time. want=after %v got=%v",
			evt.Name(), err.Events[err.EventIndex-1].Time(), evt.Time(),
		)
	default:
		return fmt.Sprintf("consistency: invalid inconsistency kind=%d", err.Kind)
	}
}

// IsConsistencyError implements error.Is.
func (err *ConsistencyError) IsConsistencyError() bool {
	return true
}

func (k ConsistencyKind) String() string {
	switch k {
	case InconsistentID:
		return "<InconsistentID>"
	case InconsistentName:
		return "<InconsistentName>"
	case InconsistentVersion:
		return "<InconsistentVersion>"
	case InconsistentTime:
		return "<InconsistentTime>"
	default:
		return "<UnknownInconsistency>"
	}
}

func currentVersion(a Aggregate) int {
	_, _, v := a.Aggregate()
	return v + len(a.AggregateChanges())
}
