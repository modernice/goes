package event

import (
	"github.com/modernice/goes/helper/streams"
)

func Filter[D any](events <-chan Of[D], queries ...Query) <-chan Of[D] {
	filters := make([]func(Of[D]) bool, len(queries))
	for i, q := range queries {
		filters[i] = func(evt Of[D]) bool {
			return Test(q, evt)
		}
	}
	return streams.Filter(events, filters...)
}
