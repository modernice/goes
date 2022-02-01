package projection_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/query"
	"github.com/modernice/goes/event/test"
	"github.com/modernice/goes/internal/projectiontest"
	"github.com/modernice/goes/projection"
)

func TestApply(t *testing.T) {
	proj := projectiontest.NewMockProjection()

	events := []event.Event{
		event.New("foo", test.FooEventData{}),
		event.New("bar", test.FooEventData{}),
		event.New("baz", test.FooEventData{}),
	}

	if err := projection.Apply(proj, events); err != nil {
		t.Fatalf("Apply failed with %q", err)
	}

	proj.ExpectApplied(t, events...)
}

func TestApply_Progressor(t *testing.T) {
	proj := projectiontest.NewMockProgressor()

	now := time.Now()
	events := []event.Event{
		event.New("foo", test.FooEventData{}, event.Time(now.Add(time.Second))),
		event.New("bar", test.FooEventData{}, event.Time(now.Add(time.Minute))),
		event.New("baz", test.FooEventData{}, event.Time(now.Add(time.Hour))),
	}

	if err := projection.Apply(proj, events); err != nil {
		t.Fatalf("Apply failed with %q", err)
	}

	p, v := proj.Progress()

	if !p.Equal(events[2].Time()) {
		t.Fatalf("Time should be %v; is %v", events[2].Time(), t)
	}

	if v != 0 {
		t.Fatalf("Version should be %v; is %v", 0, v)
	}
}

func TestApply_Progressor_version(t *testing.T) {
	proj := projectiontest.NewMockProgressor()

	now := time.Now()
	id := uuid.New()
	events := []event.Event{
		event.New("foo", test.FooEventData{}, event.Time(now.Add(time.Second)), event.Aggregate(id, "foo", 1)),
		event.New("bar", test.FooEventData{}, event.Time(now.Add(time.Minute)), event.Aggregate(id, "foo", 2)),
		event.New("baz", test.FooEventData{}, event.Time(now.Add(time.Hour)), event.Aggregate(id, "foo", 3)),
	}

	if err := projection.Apply(proj, events); err != nil {
		t.Fatalf("Apply failed with %q", err)
	}

	p, v := proj.Progress()

	if !p.Equal(events[2].Time()) {
		t.Fatalf("Time should be %v; is %v", events[2].Time(), t)
	}

	if v != 3 {
		t.Fatalf("Version should be %v; is %v", 3, v)
	}
}

func TestApply_Progressor_ErrProgressed(t *testing.T) {
	now := time.Now()
	proj := projectiontest.NewMockProgressor()
	proj.TrackProgress(now, 0)

	events := []event.Event{event.New("foo", test.FooEventData{}, event.Time(now))}

	if err := projection.Apply(proj, events); !errors.Is(err, projection.ErrProgressed) {
		t.Fatalf("Apply should fail with %q if Event time is before progress time; got %q", projection.ErrProgressed, err)
	}

	progress, version := proj.Progress()

	if !progress.Equal(now) {
		t.Fatalf("Progress should be %v; is %v", now, progress)
	}

	if version != 0 {
		t.Fatalf("Version should be %v; is %v", 0, version)
	}
}

func TestApply_Progressor_ErrProgressed_version(t *testing.T) {
	now := time.Now()
	proj := projectiontest.NewMockProgressor()
	proj.TrackProgress(now, 3)

	events := []event.Event{event.New("foo", test.FooEventData{}, event.Time(now.Add(time.Minute)), event.Aggregate(uuid.New(), "foo", 3))}

	if err := projection.Apply(proj, events); !errors.Is(err, projection.ErrProgressed) {
		t.Fatalf("Apply should fail with %q if event version is <= progress version; got %q", projection.ErrProgressed, err)
	}

	progress, version := proj.Progress()

	if !progress.Equal(now) {
		t.Fatalf("Progress should be %v; is %v", now, progress)
	}

	if version != 3 {
		t.Fatalf("Version should be %v; is %v", 3, version)
	}
}

func TestApply_Progressor_IgnoreProgress(t *testing.T) {
	now := time.Now()
	proj := projectiontest.NewMockProgressor()
	proj.TrackProgress(now, 0)

	events := []event.Event{
		event.New("foo", test.FooEventData{}, event.Time(now.Add(-time.Hour))),
		event.New("foo", test.FooEventData{}, event.Time(now.Add(-time.Minute))),
	}

	if err := projection.Apply(proj, events, projection.IgnoreProgress()); err != nil {
		t.Fatalf("Apply shouldn't fail when using IgnoreProgress; got %q", err)
	}

	if p, _ := proj.Progress(); !p.Equal(now) {
		t.Fatalf("Progress should return %v; got %v", now, p)
	}
}

func TestApply_Guard(t *testing.T) {
	guard := projection.QueryGuard(query.New(query.Name("foo", "bar")))
	proj := projectiontest.NewMockGuardedProjection(guard)

	events := []event.Event{
		event.New("foo", test.FooEventData{}),
		event.New("bar", test.FooEventData{}),
		event.New("baz", test.FooEventData{}),
		event.New("foobar", test.FooEventData{}),
	}

	if err := projection.Apply(proj, events); err != nil {
		t.Fatalf("Apply failed with %q", err)
	}

	if len(proj.AppliedEvents) != 2 {
		t.Fatalf("%d events should have been applied; got %d", 2, len(proj.AppliedEvents))
	}

	proj.ExpectApplied(t, events[:2]...)
}
