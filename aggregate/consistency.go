package aggregate

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/modernice/goes/event"
)

const (
	// InconsistentID is a ConsistencyKind indicating that an event has an
	// inconsistent AggregateID with the expected one.
	InconsistentID = ConsistencyKind(iota + 1)

	// InconsistentName is a ConsistencyKind representing an error caused by an
	// event having an invalid AggregateName that does not match the expected name
	// for the aggregate.
	InconsistentName

	// InconsistentVersion indicates an inconsistency in the version of an Event
	// within an Aggregate. This occurs when the version of an event is less than or
	// equal to the current version of the aggregate, or when the version of a new
	// event is less than or equal to the version of a previous event.
	InconsistentVersion

	// InconsistentTime indicates that an event has an invalid time, meaning it
	// occurred before its preceding event in the sequence of events being
	// validated.
	InconsistentTime
)

// ConsistencyError represents an error that occurs when the consistency of an
// aggregate's events is violated. It provides information about the kind of
// inconsistency, the aggregate reference, the current version of the aggregate,
// the list of events, and the index of the event causing the inconsistency.
type ConsistencyError struct {
	Kind           ConsistencyKind
	Aggregate      Ref
	CurrentVersion int
	Events         []event.Event
	EventIndex     int
}

// ConsistencyKind represents the kind of inconsistency found in an aggregate's
// events when validating their consistency. Possible kinds are InconsistentID,
// InconsistentName, InconsistentVersion, and InconsistentTime.
type ConsistencyKind int

// IsConsistencyError checks if the given error is a ConsistencyError or an
// error implementing the IsConsistencyError method returning true.
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

// ConsistencyOption is a functional option type used to configure consistency
// validation for event aggregates. It allows customizing the behavior of the
// ValidateConsistency function, such as ignoring time consistency checks.
type ConsistencyOption func(*consistencyValidation)

// IgnoreTime returns a ConsistencyOption that configures whether time
// consistency checks should be ignored when validating aggregate event
// consistency. If set to true, the validation will not check if events are
// ordered by their time.
func IgnoreTime(ignore bool) ConsistencyOption {
	return func(cfg *consistencyValidation) {
		cfg.ignoreTime = ignore
	}
}

type consistencyValidation struct {
	ignoreTime bool
}

// ValidateConsistency checks the consistency of the provided events with the
// given aggregate reference and current version. It returns a ConsistencyError
// if any inconsistency is found, such as mismatched IDs, names, versions, or
// event times. The consistency check can be configured with ConsistencyOption
// functions.
func ValidateConsistency[Data any, Events ~[]event.Of[Data]](ref Ref, currentVersion int, events Events, opts ...ConsistencyOption) error {
	var cfg consistencyValidation
	for _, opt := range opts {
		opt(&cfg)
	}

	aevents := make([]event.Event, len(events))
	for i, evt := range events {
		aevents[i] = event.Any(evt)
	}

	var hasPrevEvent bool
	var prevEvent event.Event
	var prevVersion int

	for i, evt := range aevents {
		eid, ename, ev := evt.Aggregate()
		if eid != ref.ID {
			return &ConsistencyError{
				Kind:           InconsistentID,
				Aggregate:      ref,
				CurrentVersion: currentVersion,
				Events:         aevents,
				EventIndex:     i,
			}
		}
		if ename != ref.Name {
			return &ConsistencyError{
				Kind:           InconsistentName,
				Aggregate:      ref,
				CurrentVersion: currentVersion,
				Events:         aevents,
				EventIndex:     i,
			}
		}
		if ev <= 0 {
			return &ConsistencyError{
				Kind:           InconsistentVersion,
				Aggregate:      ref,
				CurrentVersion: currentVersion,
				Events:         aevents,
				EventIndex:     i,
			}
		}
		if ev <= currentVersion {
			return &ConsistencyError{
				Kind:           InconsistentVersion,
				Aggregate:      ref,
				CurrentVersion: currentVersion,
				Events:         aevents,
				EventIndex:     i,
			}
		}
		if hasPrevEvent && ev <= prevVersion {
			return &ConsistencyError{
				Kind:           InconsistentVersion,
				Aggregate:      ref,
				CurrentVersion: currentVersion,
				Events:         aevents,
				EventIndex:     i,
			}
		}
		if hasPrevEvent && !cfg.ignoreTime {
			nano := evt.Time().UnixNano()
			prevNano := prevEvent.Time().UnixNano()
			if nano <= prevNano {
				return &ConsistencyError{
					Kind:           InconsistentTime,
					Aggregate:      ref,
					CurrentVersion: currentVersion,
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

// Event returns the event that caused the ConsistencyError at the EventIndex.
// If the EventIndex is out of bounds, it returns nil.
func (err *ConsistencyError) Event() event.Event {
	if err.EventIndex < 0 || err.EventIndex >= len(err.Events) {
		return nil
	}
	return err.Events[err.EventIndex]
}

// Error returns a string representation of the ConsistencyError, describing the
// inconsistency found in the aggregate event. It includes details such as the
// event name, expected and actual values for AggregateID, AggregateName,
// AggregateVersion, or Time depending on the Kind of inconsistency.
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
			"consistency: %q event has invalid AggregateVersion. want >=%d got=%d",
			evt.Name(), err.CurrentVersion+1, v,
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

// IsConsistencyError determines if the given error is a ConsistencyError or an
// error that implements the IsConsistencyError method. It returns true if
// either condition is met, otherwise false.
func (err *ConsistencyError) IsConsistencyError() bool {
	return true
}

// String returns a string representation of the ConsistencyKind value, such as
// "<InconsistentID>", "<InconsistentName>", "<InconsistentVersion>", or
// "<InconsistentTime>".
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
