package event

import (
	"github.com/modernice/goes/helper/streams"
)

func Filter[D any](events <-chan EventOf[D], queries ...Query) <-chan EventOf[D] {
	filters := make([]func(EventOf[D]) bool, len(queries))
	for i, q := range queries {
		filters[i] = func(evt EventOf[D]) bool {
			return Test(q, evt)
		}
	}
	return streams.Filter(events, filters...)
}
