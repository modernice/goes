package test

import (
	"bytes"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/modernice/goes/event"
)

// EqualEvents compares slices of Events.
func EqualEvents(events ...[]event.Event) bool {
	if len(events) < 2 {
		return true
	}
	first := events[0]
	opts := []cmp.Option{
		cmpopts.SortSlices(func(a, b event.Event) bool {
			aid := a.ID()
			bid := b.ID()
			return bytes.Compare(aid[:], bid[:]) == -1
		}),
	}
	if len(first) > 0 {
		opts = append(opts, cmp.AllowUnexported(first[0]))
	}
	for _, events := range events[1:] {
		if !cmp.Equal(first, events, opts...) {
			return false
		}
	}
	return true
}
