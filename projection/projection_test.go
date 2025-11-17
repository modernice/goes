package projection_test

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/query"
	"github.com/modernice/goes/event/test"
	"github.com/modernice/goes/internal"
	"github.com/modernice/goes/internal/projectiontest"
	"github.com/modernice/goes/internal/slice"
	"github.com/modernice/goes/projection"
)

func TestApply(t *testing.T) {
	proj := projectiontest.NewMockProjection()

	events := []event.Event{
		event.New("foo", test.FooEventData{}).Any(),
		event.New("bar", test.BarEventData{}).Any(),
		event.New("baz", test.BazEventData{}).Any(),
	}

	projection.Apply(proj, events)

	proj.ExpectApplied(t, events...)
}

func TestApply_ProgressAware(t *testing.T) {
	proj := projectiontest.NewMockProgressor()

	now := time.Now()
	events := []event.Event{
		event.New[any]("foo", test.FooEventData{}, event.Time(now.Add(time.Second))),
		event.New[any]("bar", test.FooEventData{}, event.Time(now.Add(time.Minute))),
		event.New[any]("baz", test.FooEventData{}, event.Time(now.Add(time.Hour))),
	}

	projection.Apply(proj, events)

	lastTime, ids := proj.Progress()
	if !lastTime.Equal(events[2].Time()) {
		t.Fatalf("Progress() should return %s as the last time; got %s", events[2].Time(), lastTime)
	}

	wantIDs := []uuid.UUID{events[2].ID()}
	if !cmp.Equal(ids, wantIDs) {
		t.Fatalf("Progress() should return the last applied event ids %v; got %v", wantIDs, ids)
	}
}

func TestApply_ProgressAware_multipleEventsWithSameTime(t *testing.T) {
	proj := projectiontest.NewMockProgressor()

	now := time.Now()
	evtTime := now.Add(time.Minute)
	events := []event.Event{
		event.New("foo", test.FooEventData{}, event.Time(evtTime)).Any(),
		event.New("bar", test.FooEventData{}, event.Time(evtTime)).Any(),
		event.New("baz", test.FooEventData{}, event.Time(evtTime)).Any(),
	}

	projection.Apply(proj, events)

	lastTime, ids := proj.Progress()
	if !lastTime.Equal(evtTime) {
		t.Fatalf("Progress() should return %s as the last time; got %s", evtTime, lastTime)
	}

	wantIDs := slice.Map(events, func(evt event.Event) uuid.UUID { return evt.ID() })

	if !cmp.Equal(ids, wantIDs) {
		t.Fatalf("Progress() should return the last applied event ids %v; got %v", wantIDs, ids)
	}
}

func TestApply_ProgressorAware_IgnoreProgress(t *testing.T) {
	now := time.Now()
	proj := projectiontest.NewMockProgressor()
	lastEvents := []uuid.UUID{internal.NewUUID(), internal.NewUUID()}
	proj.SetProgress(now, lastEvents...)

	events := []event.Event{
		event.New("foo", test.FooEventData{}, event.Time(now.Add(-time.Hour))).Any(),
		event.New("foo", test.FooEventData{}, event.Time(now.Add(-time.Minute))).Any(),
	}

	projection.Apply(proj, events, projection.IgnoreProgress())

	lastTime, ids := proj.Progress()
	if !lastTime.Equal(events[1].Time()) {
		t.Fatalf("Progress() should return %s as the last time; got %s", events[1].Time(), lastTime)
	}

	wantIDs := []uuid.UUID{events[1].ID()}
	if !cmp.Equal(ids, wantIDs) {
		t.Fatalf("Progress() should return the last applied event ids %v; got %v", wantIDs, ids)
	}
}

func TestApply_Guard(t *testing.T) {
	guard := projection.QueryGuard(query.New(query.Name("foo", "bar")))
	proj := projectiontest.NewMockGuardedProjection(guard)

	events := []event.Event{
		event.New[any]("foo", test.FooEventData{}),
		event.New[any]("bar", test.FooEventData{}),
		event.New[any]("baz", test.FooEventData{}),
		event.New[any]("foobar", test.FooEventData{}),
	}

	projection.Apply(proj, events)

	if len(proj.AppliedEvents) != 2 {
		t.Fatalf("%d events should have been applied; got %d", 2, len(proj.AppliedEvents))
	}

	proj.ExpectApplied(t, events[:2]...)
}
