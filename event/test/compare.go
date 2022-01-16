package test

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/modernice/goes/event"
)

type eventSlice[D any] []event.Event[D]

// EqualEvents compares slices of Events.
func EqualEvents[D any, Events ~[]event.Event[D]](events ...Events) bool {
	if len(events) < 2 {
		return true
	}
	for i, evts := range events {
		if len(evts) == 0 {
			events[i] = nil
		}
	}
	first := events[0]
	var opts []cmp.Option
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

// AssertEqualEvents compares slices of events and reports an error to
// testing.T if they don't match.
func AssertEqualEvents[D any, Events ~[]event.Event[D]](t *testing.T, events ...Events) {
	if len(events) < 2 {
		return
	}
	if EqualEvents[D, Events](events...) {
		return
	}

	var msg strings.Builder
	_, err := msg.WriteString("events don't match:\n\n")
	if err != nil {
		t.Fatal(fmt.Errorf("msg.WriteString: %w", err))
	}

	for i, events := range events {
		msg.WriteString(fmt.Sprintf("[%d]: %#v\n\n", i, events))
	}

	first := events[0]
	var opts []cmp.Option
	if len(first) > 0 {
		opts = append(opts, cmp.AllowUnexported(first[0]))
	}

	for i, events := range events[1:] {
		if diff := cmp.Diff(first, events, opts...); diff != "" {
			msg.WriteString(fmt.Sprintf("[%d <-> %d] diff: %s", 0, i+1, diff))
		}
	}

	t.Error(msg.String())
}

// AssertEqualEventsUnsorted does the same as AssertEqualEvents but ignores
// the order of the events.
func AssertEqualEventsUnsorted[D any, Events ~[]event.Event[D]](t *testing.T, events ...Events) {
	for i, evts := range events {
		es := eventSlice[D](evts)
		es.sortByTime()
		events[i] = Events(es)
	}
	AssertEqualEvents[D, Events](t, events...)
}

func (es eventSlice[D]) sortByTime() {
	sort.Slice(es, func(i, j int) bool {
		return es[i].Time().Equal(es[j].Time()) ||
			es[i].Time().Before(es[j].Time())
	})
}
