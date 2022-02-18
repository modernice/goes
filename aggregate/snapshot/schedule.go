package snapshot

import (
	"github.com/modernice/goes"
	"github.com/modernice/goes/aggregate"
)

// A Schedule determines if an Aggregate is scheduled to be snapshotted.
type Schedule[ID goes.ID] interface {
	// Test returns true if the given Aggregate should be snapshotted.
	Test(aggregate.AggregateOf[ID]) bool
}

type scheduleFunc[ID goes.ID] func(aggregate.AggregateOf[ID]) bool

// Every returns a Schedule that instructs to make Snapshots of an Aggregate
// every nth Event of the Aggregate.
func Every[ID goes.ID](n int) Schedule[ID] {
	return scheduleFunc[ID](func(a aggregate.AggregateOf[ID]) bool {
		_, _, old := a.Aggregate()
		current := aggregate.UncommittedVersion(a)

		for v := old + 1; v <= current; v++ {
			if v%n == 0 {
				return true
			}
		}

		return false
	})
}

func (fn scheduleFunc[ID]) Test(a aggregate.AggregateOf[ID]) bool {
	return fn(a)
}
