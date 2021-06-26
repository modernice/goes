package projection_test

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/query"
	"github.com/modernice/goes/event/test"
	"github.com/modernice/goes/projection"
)

func TestApply(t *testing.T) {
	proj := newMockProjection()

	events := []event.Event{
		event.New("foo", test.FooEventData{}),
		event.New("bar", test.FooEventData{}),
		event.New("baz", test.FooEventData{}),
	}

	if err := projection.Apply(proj, events); err != nil {
		t.Fatalf("Apply failed with %q", err)
	}

	proj.expectApplied(t, events...)
}

func TestApply_Progressor(t *testing.T) {
	proj := newMockProgressor()

	now := time.Now()
	evtTime := now.Add(time.Minute)
	events := []event.Event{
		event.New("foo", test.FooEventData{}),
		event.New("bar", test.FooEventData{}),
		event.New("baz", test.FooEventData{}, event.Time(evtTime)),
	}

	if err := projection.Apply(proj, events); err != nil {
		t.Fatalf("Apply failed with %q", err)
	}

	if !proj.Progress().Equal(evtTime) {
		t.Fatalf("Progress should return %v; got %v", evtTime, proj.Progress())
	}
}

func TestApply_Guard(t *testing.T) {
	guard := projection.Guard(query.New(query.Name("foo", "bar")))
	proj := newMockGuardedProjection(guard)

	events := []event.Event{
		event.New("foo", test.FooEventData{}),
		event.New("bar", test.FooEventData{}),
		event.New("baz", test.FooEventData{}),
		event.New("foobar", test.FooEventData{}),
	}

	if err := projection.Apply(proj, events); !errors.Is(err, projection.ErrGuarded) {
		t.Fatalf("Apply should fail with %q; got %q", projection.ErrGuarded, err)
	}

	if len(proj.appliedEvents) != 2 {
		t.Fatalf("%d events should have been applied; got %d", 2, len(proj.appliedEvents))
	}

	proj.expectApplied(t, events[:2]...)
}

type mockProjection struct {
	appliedEvents []event.Event
}

func newMockProjection() *mockProjection {
	return &mockProjection{}
}

func (proj *mockProjection) ApplyEvent(evt event.Event) {
	proj.appliedEvents = append(proj.appliedEvents, evt)
}

func (proj *mockProjection) hasApplied(events ...event.Event) bool {
	for _, evt := range events {
		var applied bool
		for _, pevt := range proj.appliedEvents {
			if reflect.DeepEqual(evt, pevt) {
				applied = true
				break
			}
		}
		if !applied {
			return false
		}
	}
	return true
}

func (proj *mockProjection) expectApplied(t *testing.T, events ...event.Event) {
	if !proj.hasApplied(events...) {
		t.Fatalf("mockProjection should have applied %v; has applied %v", events, proj.appliedEvents)
	}
}

type mockProgressor struct {
	*mockProjection
	*projection.Progressor
}

func newMockProgressor() *mockProgressor {
	return &mockProgressor{
		mockProjection: newMockProjection(),
		Progressor:     &projection.Progressor{},
	}
}

type mockGuardedProjection struct {
	*mockProjection
	projection.Guard
}

func newMockGuardedProjection(guard projection.Guard) *mockGuardedProjection {
	return &mockGuardedProjection{
		mockProjection: newMockProjection(),
		Guard:          guard,
	}
}
