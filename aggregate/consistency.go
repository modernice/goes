package aggregate

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/modernice/goes/event"
)

// ConsistencyKind describes why a set of events is inconsistent.
type ConsistencyKind int

const (
	// InconsistentID means an event carries a different aggregate id.
	InconsistentID ConsistencyKind = iota + 1
	// InconsistentName means an event carries a different aggregate name.
	InconsistentName
	// InconsistentVersion means versions are non monotonically increasing.
	InconsistentVersion
	// InconsistentTime means event times go backwards.
	InconsistentTime
)

// ConsistencyError reports a violation found while validating a sequence of
// events for an aggregate.
type ConsistencyError struct {
	Kind           ConsistencyKind
	Aggregate      Ref
	CurrentVersion int
	Events         []event.Event
	EventIndex     int
}

// IsConsistencyError returns true if err is or wraps a ConsistencyError.
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

// ConsistencyOption configures ValidateConsistency.
type ConsistencyOption func(*consistencyValidation)

// IgnoreTime skips event time validation when set to true.
func IgnoreTime(ignore bool) ConsistencyOption {
	return func(cfg *consistencyValidation) { cfg.ignoreTime = ignore }
}

type consistencyValidation struct{ ignoreTime bool }

// ValidateConsistency checks whether events belong to the provided aggregate
// reference and advance version and time monotonically.
func ValidateConsistency[Data any, Events ~[]event.Of[Data]](ref Ref, currentVersion int, events Events, opts ...ConsistencyOption) error {
	var cfg consistencyValidation
	for _, opt := range opts {
		opt(&cfg)
	}
	aevents := make([]event.Event, len(events))
	for i, evt := range events {
		aevents[i] = event.Any(evt)
	}
	var hasPrev bool
	var prev event.Event
	var prevVersion int
	for i, evt := range aevents {
		eid, ename, ev := evt.Aggregate()
		if eid != ref.ID {
			return &ConsistencyError{Kind: InconsistentID, Aggregate: ref, CurrentVersion: currentVersion, Events: aevents, EventIndex: i}
		}
		if ename != ref.Name {
			return &ConsistencyError{Kind: InconsistentName, Aggregate: ref, CurrentVersion: currentVersion, Events: aevents, EventIndex: i}
		}
		if ev <= 0 || ev <= currentVersion || (hasPrev && ev <= prevVersion) {
			return &ConsistencyError{Kind: InconsistentVersion, Aggregate: ref, CurrentVersion: currentVersion, Events: aevents, EventIndex: i}
		}
		if hasPrev && !cfg.ignoreTime {
			if evt.Time().UnixNano() <= prev.Time().UnixNano() {
				return &ConsistencyError{Kind: InconsistentTime, Aggregate: ref, CurrentVersion: currentVersion, Events: aevents, EventIndex: i}
			}
		}
		prev = evt
		prevVersion = ev
		hasPrev = true
	}
	return nil
}

// Event returns the offending event or nil if the index is out of range.
func (err *ConsistencyError) Event() event.Event {
	if err.EventIndex < 0 || err.EventIndex >= len(err.Events) {
		return nil
	}
	return err.Events[err.EventIndex]
}

// Error implements error.
func (err *ConsistencyError) Error() string {
	evt := err.Event()
	var id uuid.UUID
	var name string
	var v int
	if evt != nil {
		id, name, v = evt.Aggregate()
	}
	var aid uuid.UUID
	var aname string
	if err.Aggregate != (Ref{}) {
		aid, aname = err.Aggregate.ID, err.Aggregate.Name
	}
	switch err.Kind {
	case InconsistentID:
		return fmt.Sprintf("inconsistent aggregate id: have %s want %s", id, aid)
	case InconsistentName:
		return fmt.Sprintf("inconsistent aggregate name: have %s want %s", name, aname)
	case InconsistentVersion:
		return fmt.Sprintf("inconsistent aggregate version: have %d after %d", v, err.CurrentVersion)
	case InconsistentTime:
		return fmt.Sprintf("inconsistent event time: %s is before previous event", evt.Time())
	default:
		return "invalid aggregate events"
	}
}
