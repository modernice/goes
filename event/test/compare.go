package test

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/modernice/goes/event"
)

type eventSlice []event.Event

// EqualEvents compares slices of Events.
func EqualEvents(events ...[]event.Event) bool {
	if len(events) < 2 {
		return true
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
func AssertEqualEvents(t *testing.T, events ...[]event.Event) {
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

	for i, events := range events[1:] {
		if diff := cmp.Diff(first, events, opts...); diff != "" {
			msg.WriteString(fmt.Sprintf("[%d <-> %d] diff: %s", 0, i+1, diff))
		}
	}

	t.Error(msg.String())
}

// AssertEqualEventsUnsorted does the same as AssertEqualEvents but ignores
// the order of the events.
func AssertEqualEventsUnsorted(t *testing.T, events ...[]event.Event) {
	for i, evts := range events {
		es := eventSlice(evts)
		es.sortByTime()
		events[i] = []event.Event(es)
	}
	AssertEqualEvents(t, events...)
}

func (es eventSlice) sortByTime() {
	sort.Slice(es, func(i, j int) bool {
		return es[i].Time().Equal(es[j].Time()) ||
			es[i].Time().Before(es[j].Time())
	})
}
