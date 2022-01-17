package xaggregate

import (
	"math/rand"
	"time"

	"github.com/modernice/goes/aggregate"
)

// Shuffle shuffles aggregates and returns the shuffled aggregates.
func Shuffle(as []aggregate.Aggregate[any]) []aggregate.Aggregate[any] {
	shuffled := make([]aggregate.Aggregate[any], len(as))
	copy(shuffled, as)
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(shuffled), func(i, j int) {
		shuffled[j], shuffled[i] = shuffled[i], shuffled[j]
	})
	return shuffled
}
