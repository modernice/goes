package test

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/modernice/goes/event"
)

type eventSlice[D any] []event.Of[D]

// EqualEvents compares slices of events.
func EqualEvents[Events ~[]event.Of[D], D any](events ...Events) bool {
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
func AssertEqualEvents[Events ~[]event.Of[D], D any](t *testing.T, events ...Events) {
	if len(events) < 2 {
		return
	}
	if EqualEvents(events...) {
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
func AssertEqualEventsUnsorted[Events ~[]event.Of[D], D any](t *testing.T, events ...Events) {
	normalized := make([]Events, len(events))
	for i, evts := range events {
		es := make(eventSlice[D], len(evts))
		copy(es, evts)
		es.sortByID()
		normalized[i] = Events(es)
	}
	AssertEqualEvents(t, normalized...)
}

// sortByID sorts the events into a canonical order. The event id is the sort
// key because it is the only property that is guaranteed to be unique; event
// times can collide, which would make the normalized order depend on the
// input order.
func (es eventSlice[D]) sortByID() {
	sort.Slice(es, func(i, j int) bool {
		a, b := es[i].ID(), es[j].ID()
		return bytes.Compare(a[:], b[:]) < 0
	})
}
