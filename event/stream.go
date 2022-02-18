package event

import (
	"github.com/modernice/goes"
	"github.com/modernice/goes/helper/streams"
)

func Filter[D any, ID goes.ID](events <-chan Of[D, ID], queries ...QueryOf[ID]) <-chan Of[D, ID] {
	filters := make([]func(Of[D, ID]) bool, len(queries))
	for i, q := range queries {
		filters[i] = func(evt Of[D, ID]) bool {
			return Test(q, evt)
		}
	}
	return streams.Filter(events, filters...)
}
