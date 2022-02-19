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

	events := []event.Of[any, uuid.UUID]{
		event.New[any](uuid.New(), "foo", test.FooEventData{}),
		event.New[any](uuid.New(), "bar", test.FooEventData{}),
		event.New[any](uuid.New(), "baz", test.FooEventData{}),
	}

	if err := projection.Apply[uuid.UUID](proj, events); err != nil {
		t.Fatalf("Apply failed with %q", err)
	}

	proj.ExpectApplied(t, events...)
}

func TestApply_Progressor(t *testing.T) {
	proj := projectiontest.NewMockProgressor()

	now := time.Now()
	events := []event.Of[any, uuid.UUID]{
		event.New[any](uuid.New(), "foo", test.FooEventData{}, event.Time(now.Add(time.Second))),
		event.New[any](uuid.New(), "bar", test.FooEventData{}, event.Time(now.Add(time.Minute))),
		event.New[any](uuid.New(), "baz", test.FooEventData{}, event.Time(now.Add(time.Hour))),
	}

	if err := projection.Apply[uuid.UUID](proj, events); err != nil {
		t.Fatalf("Apply failed with %q", err)
	}

	if !proj.Progress().Equal(events[2].Time()) {
		t.Fatalf("Progress should return %v; got %v", events[2].Time(), proj.Progress())
	}
}

func TestApply_Progressor_ErrProgressed(t *testing.T) {
	now := time.Now()
	proj := projectiontest.NewMockProgressor()
	proj.SetProgress(now)

	events := []event.Of[any, uuid.UUID]{event.New[any](uuid.New(), "foo", test.FooEventData{}, event.Time(now))}

	if err := projection.Apply[uuid.UUID](proj, events); !errors.Is(err, projection.ErrProgressed) {
		t.Fatalf("Apply should fail with %q if Event time is before progress time; got %q", projection.ErrProgressed, err)
	}

	if !proj.Progress().Equal(now) {
		t.Fatalf("Progress should return %v; got %v", now, proj.Progress())
	}
}

func TestApply_Progressor_IgnoreProgress(t *testing.T) {
	now := time.Now()
	proj := projectiontest.NewMockProgressor()
	proj.SetProgress(now)

	events := []event.Of[any, uuid.UUID]{
		event.New[any](uuid.New(), "foo", test.FooEventData{}, event.Time(now.Add(-time.Hour))),
		event.New[any](uuid.New(), "foo", test.FooEventData{}, event.Time(now.Add(-time.Minute))),
	}

	if err := projection.Apply[uuid.UUID](proj, events, projection.IgnoreProgress()); err != nil {
		t.Fatalf("Apply shouldn't fail when using IgnoreProgress; got %q", err)
	}

	if !proj.Progress().Equal(now) {
		t.Fatalf("Progress should return %v; got %v", now, proj.Progress())
	}
}

func TestApply_Guard(t *testing.T) {
	guard := projection.QueryGuard(query.New[uuid.UUID](query.Name("foo", "bar")))
	proj := projectiontest.NewMockGuardedProjection(guard)

	events := []event.Of[any, uuid.UUID]{
		event.New[any](uuid.New(), "foo", test.FooEventData{}),
		event.New[any](uuid.New(), "bar", test.FooEventData{}),
		event.New[any](uuid.New(), "baz", test.FooEventData{}),
		event.New[any](uuid.New(), "foobar", test.FooEventData{}),
	}

	if err := projection.Apply[uuid.UUID](proj, events); err != nil {
		t.Fatalf("Apply failed with %q", err)
	}

	if len(proj.AppliedEvents) != 2 {
		t.Fatalf("%d events should have been applied; got %d", 2, len(proj.AppliedEvents))
	}

	proj.ExpectApplied(t, events[:2]...)
}
