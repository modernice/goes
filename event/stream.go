package event

import (
	"github.com/modernice/goes/helper/streams"
)

// Filter filters an event stream using the provided query. The returned stream
// only includes events that match the query.
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
