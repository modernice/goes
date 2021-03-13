package snapshot

import (
	"github.com/modernice/goes/aggregate"
)

// A Schedule determines if an Aggregate is scheduled to be snapshotted.
type Schedule interface {
	// Test returns true if the given Aggregate should be snapshotted.
	Test(aggregate.Aggregate) bool
}

type scheduleFunc func(aggregate.Aggregate) bool

// Every returns a Schedule that instructs to make Snapshots of an Aggregate
// every nth Event of that Aggregate.
func Every(n int) Schedule {
	return scheduleFunc(func(a aggregate.Aggregate) bool {
		old := a.AggregateVersion()
		current := aggregate.CurrentVersion(a)

		for v := old + 1; v <= current; v++ {
			if v%n == 0 {
				return true
			}
		}

		return false
	})
}

func (fn scheduleFunc) Test(a aggregate.Aggregate) bool {
	return fn(a)
}
