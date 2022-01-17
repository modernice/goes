package snapshot

import (
	"github.com/modernice/goes/aggregate"
)

// A Schedule determines if an Aggregate is scheduled to be snapshotted.
type Schedule[D any] interface {
	// Test returns true if the given Aggregate should be snapshotted.
	Test(aggregate.Aggregate[D]) bool
}

type scheduleFunc[D any] func(aggregate.Aggregate[D]) bool

// Every returns a Schedule that instructs to make Snapshots of an Aggregate
// every nth Event of that Aggregate.
func Every[D any](n int) Schedule[D] {
	return scheduleFunc[D](func(a aggregate.Aggregate[D]) bool {
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

func (fn scheduleFunc[D]) Test(a aggregate.Aggregate[D]) bool {
	return fn(a)
}
