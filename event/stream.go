package event

import (
	"github.com/modernice/goes/helper/streams"
)

// Filter filters the events from the given input channel based on the provided
// queries and returns a new channel with only the events that pass all the
// queries. If no queries are provided, the input channel is returned unchanged.
func Filter[D any](events <-chan Of[D], queries ...Query) <-chan Of[D] {
	if len(queries) == 0 {
		return events
	}

	filters := make([]func(Of[D]) bool, len(queries))
	for i, q := range queries {
		filters[i] = func(evt Of[D]) bool {
			return Test(q, evt)
		}
	}

	return streams.Filter(events, filters...)
}
