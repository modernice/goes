package xevent

import (
	"math/rand"
	"time"

	"github.com/modernice/goes"
	"github.com/modernice/goes/event"
)

// Shuffle shuffles events and returns the shuffled slice.
func Shuffle[D any, ID goes.ID](events []event.Of[D, ID]) []event.Of[D, ID] {
	shuffled := make([]event.Of[D, ID], len(events))
	copy(shuffled, events)
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(shuffled), func(i, j int) {
		shuffled[j], shuffled[i] = shuffled[i], shuffled[j]
	})
	return shuffled
}
