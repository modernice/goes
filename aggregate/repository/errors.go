package repository

import (
	"errors"

	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/event"
)

// SaveError is an error that happened while saving an Aggregate.
type SaveError struct {
	// Aggregate is the Aggregate that failed to save.
	Aggregate aggregate.Aggregate
	// Err is the actual error.
	Err error
	// Rollbacks contains the rollback results.
	Rollbacks SaveRollbacks
}

// SaveRollbacks contains the rollback results for a failed save.
type SaveRollbacks []struct {
	Event event.Event
	Err   error
}

func (err *SaveError) Error() string {
	return ""
}

// Is determines if err and target are equal.
func (err *SaveError) Is(target error) bool {
	se := &SaveError{}
	if !errors.As(target, &se) {
		return false
	}

	if err == se {
		return true
	}

	if err.Aggregate.AggregateID() != se.Aggregate.AggregateID() ||
		err.Aggregate.AggregateName() != se.Aggregate.AggregateName() ||
		err.Aggregate.AggregateVersion() != se.Aggregate.AggregateVersion() {
		return false
	}

	if len(err.Rollbacks) != len(se.Rollbacks) {
		return false
	}

	for i, rb := range err.Rollbacks {
		if !event.Equal(rb.Event, se.Rollbacks[i].Event) {
			return false
		}

		if rb.Err == nil && se.Rollbacks[i].Err != nil {
			return false
		} else if rb.Err != nil && se.Rollbacks[i].Err == nil {
			return false
		}
	}

	return true
}
