package aggregate

import (
	"fmt"

	"github.com/modernice/goes"
	"github.com/modernice/goes/event"
)

const (
	// ID means there is an inconsistency in the ID of an Aggregate.
	InconsistentID = ConsistencyKind(iota + 1)

	// Name means there is an inconsistency in the Aggregate names of the Events
	// of an Aggregate.
	InconsistentName

	// Version means there is an inconsistency in the Event versions of an
	// Aggregate.
	InconsistentVersion

	// Time means there is an inconsistency in the Event times of an Aggregate.
	InconsistentTime
)

// Error is a consistency error.
type ConsistencyError[ID goes.ID] struct {
	// Kind is the kind of incosistency.
	Kind ConsistencyKind
	// Aggregate is the handled aggregate.
	Aggregate AggregateOf[ID]
	// Events are the tested events.
	Events []event.Of[any, ID]
	// EventIndex is the index of the Event that caused the Error.
	EventIndex int
}

// ConsistencyKind is the kind of inconsistency.
type ConsistencyKind int

// Validate tests the consistency of the given events against the given aggregate.
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
func ValidateConsistency[ID goes.ID, Data any, Events ~[]event.Of[Data, ID]](a AggregateOf[ID], events Events) error {
	id, name, _ := a.Aggregate()
	version := currentVersion(a)
	cv := version
	var prev event.Of[any, ID]
	var hasPrev bool

	aevents := make([]event.Of[any, ID], len(events))
	for i, evt := range events {
		aevents[i] = event.ToAny(evt)
	}

	for i, evt := range aevents {
		eid, ename, ev := evt.Aggregate()
		if eid != id {
			return &ConsistencyError[ID]{
				Kind:       InconsistentID,
				Aggregate:  a,
				Events:     aevents,
				EventIndex: i,
			}
		}
		if ename != name {
			return &ConsistencyError[ID]{
				Kind:       InconsistentName,
				Aggregate:  a,
				Events:     aevents,
				EventIndex: i,
			}
		}
		if ev != cv+1 {
			return &ConsistencyError[ID]{
				Kind:       InconsistentVersion,
				Aggregate:  a,
				Events:     aevents,
				EventIndex: i,
			}
		}
		if hasPrev {
			nano := evt.Time().UnixNano()
			prevNano := prev.Time().UnixNano()
			if nano <= prevNano {
				return &ConsistencyError[ID]{
					Kind:       InconsistentTime,
					Aggregate:  a,
					Events:     aevents,
					EventIndex: i,
				}
			}
		}
		prev = evt
		hasPrev = true
		cv++
	}
	return nil
}

// Event return the first Event that caused an inconsistency.
func (err *ConsistencyError[ID]) Event() event.Of[any, ID] {
	if err.EventIndex < 0 || err.EventIndex >= len(err.Events) {
		return nil
	}
	return err.Events[err.EventIndex]
}

func (err *ConsistencyError[ID]) Error() string {
	evt := err.Event()
	var (
		id   ID
		name string
		v    int
	)
	if evt != nil {
		id, name, v = evt.Aggregate()
	}

	var (
		aid   ID
		aname string
	)

	if err.Aggregate != nil {
		aid, aname, _ = err.Aggregate.Aggregate()
	}

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
			evt.Name(), currentVersion(err.Aggregate)+1+err.EventIndex, v,
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

func (k ConsistencyKind) String() string {
	switch k {
	case InconsistentID:
		return "<InconsistentID>"
	case InconsistentName:
		return "<InconsistentName>"
	case InconsistentVersion:
		return "<InconsistentVersion>"
	case InconsistentTime:
		return "<InconsitentTime>"
	default:
		return "<UnknownInconsitency>"
	}
}

func currentVersion[ID goes.ID](a AggregateOf[ID]) int {
	_, _, v := a.Aggregate()
	return v + len(a.AggregateChanges())
}
