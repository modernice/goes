package schedule_test

import (
	"context"
	"testing"
	"time"

	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventbus"
	"github.com/modernice/goes/event/eventstore"
	"github.com/modernice/goes/event/query"
	"github.com/modernice/goes/event/test"
	"github.com/modernice/goes/internal/projectiontest"
	"github.com/modernice/goes/projection"
	"github.com/modernice/goes/projection/schedule"
)

func TestContinuous_Subscribe(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus := eventbus.New()
	store := eventstore.New()

	schedule := schedule.Continuously(bus, store, []string{"foo", "bar", "baz"})

	proj := projectiontest.NewMockProjection()
	applyErrors := make(chan error)
	appliedJobs := make(chan projection.Job)

	errs, err := schedule.Subscribe(ctx, func(job projection.Job) error {
		if err := job.Apply(job, proj); err != nil {
			applyErrors <- err
		}
		appliedJobs <- job
		return nil
	})
	if err != nil {
		t.Fatalf("Subscribe failed with %q", err)
	}

	events := []event.Event{
		event.New("foo", test.FooEventData{}),
		event.New("bar", test.FooEventData{}),
		event.New("baz", test.FooEventData{}),
		event.New("foobar", test.FooEventData{}),
	}

	if err := bus.Publish(ctx, events...); err != nil {
		t.Fatalf("publish Events: %v", err)
	}

	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()

	var applyCount int
L:
	for {
		select {
		case <-timer.C:
			t.Fatalf("timed out. applyCount=%d/3", applyCount)
		case err := <-errs:
			t.Fatal(err)
		case err := <-applyErrors:
			t.Fatal(err)
		case <-appliedJobs:
			applyCount++
			if applyCount == 3 {
				select {
				case <-appliedJobs:
					t.Fatalf("only 3 Jobs should be created; got at least 4")
				case <-time.After(100 * time.Millisecond):
					break L
				}
			}
		}
	}

	proj.ExpectApplied(t, events[:3]...)
}

func TestContinuous_Subscribe_Debounce(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus := eventbus.New()
	store := eventstore.New()

	schedule := schedule.Continuously(bus, store, []string{"foo", "bar", "baz"}, schedule.Debounce(100*time.Millisecond))

	proj := projectiontest.NewMockProjection()
	applyErrors := make(chan error)
	appliedJobs := make(chan projection.Job)

	errs, err := schedule.Subscribe(ctx, func(job projection.Job) error {
		if err := job.Apply(job, proj); err != nil {
			applyErrors <- err
		}
		appliedJobs <- job
		return nil
	})
	if err != nil {
		t.Fatalf("Subscribe failed with %q", err)
	}

	events := []event.Event{
		event.New("foo", test.FooEventData{}),
		event.New("bar", test.FooEventData{}),
		event.New("baz", test.FooEventData{}),
		event.New("foobar", test.FooEventData{}),
	}

	if err := bus.Publish(ctx, events...); err != nil {
		t.Fatalf("publish Events: %v", err)
	}

	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()

L:
	for {
		select {
		case <-timer.C:
			t.Fatalf("timed out")
		case err := <-errs:
			t.Fatal(err)
		case err := <-applyErrors:
			t.Fatal(err)
		case <-appliedJobs:
			select {
			case <-appliedJobs:
				t.Fatalf("only 1 Job should be created; got at least 2")
			case <-time.After(100 * time.Millisecond):
				break L
			}
		}
	}

	proj.ExpectApplied(t, events[:3]...)
}

func TestContinuous_Subscribe_Progressor(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus := eventbus.New()
	store := eventstore.New()

	schedule := schedule.Continuously(bus, store, []string{"foo", "bar", "baz"}, schedule.Debounce(100*time.Millisecond))

	proj := projectiontest.NewMockProgressor()
	now := time.Now()
	proj.SetProgress(now)

	applyErrors := make(chan error)
	appliedJobs := make(chan projection.Job)

	errs, err := schedule.Subscribe(ctx, func(job projection.Job) error {
		if err := job.Apply(job, proj); err != nil {
			applyErrors <- err
		}
		appliedJobs <- job
		return nil
	})
	if err != nil {
		t.Fatalf("Subscribe failed with %q", err)
	}

	events := []event.Event{
		event.New("foo", test.FooEventData{}, event.Time(now.Add(-time.Minute))),
		event.New("bar", test.FooEventData{}, event.Time(now)),
		event.New("baz", test.FooEventData{}, event.Time(now.Add(time.Second))),
		event.New("foobar", test.FooEventData{}, event.Time(now.Add(time.Minute))),
	}

	if err := bus.Publish(ctx, events...); err != nil {
		t.Fatalf("publish Events: %v", err)
	}

	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()

L:
	for {
		select {
		case <-timer.C:
			t.Fatalf("timed out")
		case err := <-errs:
			t.Fatal(err)
		case err := <-applyErrors:
			t.Fatal(err)
		case <-appliedJobs:
			select {
			case <-appliedJobs:
				t.Fatalf("only 1 Job should be created; got at least 2")
			case <-time.After(100 * time.Millisecond):
				break L
			}
		}
	}

	test.AssertEqualEvents(t, proj.AppliedEvents, events[2:3])
}

func TestContinuous_Trigger(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus := eventbus.New()
	store := eventstore.New()

	storeEvents := []event.Event{
		event.New("foo", test.FooEventData{}),
		event.New("bar", test.FooEventData{}),
		event.New("baz", test.FooEventData{}),
		event.New("foobar", test.FooEventData{}),
	}

	if err := store.Insert(ctx, storeEvents...); err != nil {
		t.Fatalf("insert Events: %v", err)
	}

	schedule := schedule.Continuously(bus, store, []string{"foo", "bar", "baz"})

	proj := projectiontest.NewMockProjection()

	applied := make(chan struct{})

	errs, err := schedule.Subscribe(ctx, func(job projection.Job) error {
		if err := job.Apply(job, proj); err != nil {
			return err
		}
		close(applied)
		return nil
	})
	if err != nil {
		t.Fatalf("Subscribe failed with %q", err)
	}

	triggerError := make(chan error)
	go func() {
		if err := schedule.Trigger(ctx); err != nil {
			triggerError <- err
		}
	}()

	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
	select {
	case <-timer.C:
		t.Fatal("timed out")
	case err := <-errs:
		t.Fatal(err)
	case err := <-triggerError:
		t.Fatal(err)
	case <-applied:
	}

	proj.ExpectApplied(t, storeEvents[:3]...)
}

func TestContinuous_Trigger_Filter(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus := eventbus.New()
	store := eventstore.New()

	storeEvents := []event.Event{
		event.New("foo", test.FooEventData{}),
		event.New("bar", test.FooEventData{}),
		event.New("baz", test.FooEventData{}),
		event.New("foobar", test.FooEventData{}),
	}

	if err := store.Insert(ctx, storeEvents...); err != nil {
		t.Fatalf("insert Events: %v", err)
	}

	sch := schedule.Continuously(bus, store, []string{"foo", "bar", "baz", "foobar"})

	proj := projectiontest.NewMockProjection()

	applied := make(chan struct{})

	errs, err := sch.Subscribe(ctx, func(job projection.Job) error {
		if err := job.Apply(job, proj); err != nil {
			return err
		}
		close(applied)
		return nil
	})
	if err != nil {
		t.Fatalf("Subscribe failed with %q", err)
	}

	triggerError := make(chan error)
	go func() {
		if err := sch.Trigger(ctx, projection.Filter(
			query.New(query.Name("bar", "baz", "foobar")),
			query.New(query.Name("bar", "foobar")),
		)); err != nil {
			triggerError <- err
		}
	}()

	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
	select {
	case <-timer.C:
		t.Fatal("timed out")
	case err := <-errs:
		t.Fatal(err)
	case err := <-triggerError:
		t.Fatal(err)
	case <-applied:
	}

	if len(proj.AppliedEvents) != 2 {
		t.Fatalf("%d Events should be applied; got %d", 2, len(proj.AppliedEvents))
	}

	proj.ExpectApplied(t, storeEvents[1], storeEvents[3])
}

func TestContinuous_Trigger_Query(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus := eventbus.New()
	store := eventstore.New()

	storeEvents := []event.Event{
		event.New("foo", test.FooEventData{}),
		event.New("bar", test.FooEventData{}),
		event.New("baz", test.FooEventData{}),
		event.New("foobar", test.FooEventData{}),
	}

	if err := store.Insert(ctx, storeEvents...); err != nil {
		t.Fatalf("insert Events: %v", err)
	}

	sch := schedule.Continuously(bus, store, []string{"foo", "bar", "baz", "foobar"})

	proj := projectiontest.NewMockProjection()

	applied := make(chan struct{})

	errs, err := sch.Subscribe(ctx, func(job projection.Job) error {
		if err := job.Apply(job, proj); err != nil {
			return err
		}
		close(applied)
		return nil
	})
	if err != nil {
		t.Fatalf("Subscribe failed with %q", err)
	}

	triggerError := make(chan error)
	go func() {
		if err := sch.Trigger(ctx, projection.Query(
			query.New(query.Name("bar", "baz", "foobar")),
		)); err != nil {
			triggerError <- err
		}
	}()

	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
	select {
	case <-timer.C:
		t.Fatal("timed out")
	case err := <-errs:
		t.Fatal(err)
	case err := <-triggerError:
		t.Fatal(err)
	case <-applied:
	}

	if len(proj.AppliedEvents) != 3 {
		t.Fatalf("%d Events should be applied; got %d", 3, len(proj.AppliedEvents))
	}

	proj.ExpectApplied(t, storeEvents[1], storeEvents[2], storeEvents[3])
}

func TestContinuous_Trigger_Reset(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus := eventbus.New()
	store := eventstore.New()

	now := time.Now()
	storeEvents := []event.Event{
		event.New("foo", test.FooEventData{}, event.Time(now)),
		event.New("bar", test.FooEventData{}, event.Time(now.Add(time.Second))),
		event.New("baz", test.FooEventData{}, event.Time(now.Add(time.Minute))),
		event.New("foobar", test.FooEventData{}, event.Time(now.Add(time.Hour))),
	}

	if err := store.Insert(ctx, storeEvents...); err != nil {
		t.Fatalf("insert Events: %v", err)
	}

	sch := schedule.Continuously(bus, store, []string{"foo", "bar", "baz", "foobar"})

	proj := projectiontest.NewMockResetProjection(7)

	if err := projection.Apply(proj, storeEvents); err != nil {
		t.Fatalf("apply projection: %v", err)
	}

	applied := make(chan struct{})

	errs, err := sch.Subscribe(ctx, func(job projection.Job) error {
		if err := job.Apply(job, proj); err != nil {
			return err
		}
		close(applied)
		return nil
	})
	if err != nil {
		t.Fatalf("Subscribe failed with %q", err)
	}

	triggerError := make(chan error)
	go func() {
		if err := sch.Trigger(ctx, projection.Reset()); err != nil {
			triggerError <- err
		}
	}()

	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
	select {
	case <-timer.C:
		t.Fatal("timed out")
	case err := <-errs:
		t.Fatal(err)
	case err := <-triggerError:
		t.Fatal(err)
	case <-applied:
	}

	if len(proj.AppliedEvents) != 4 {
		t.Fatalf("%d Events should be applied; got %d", 4, len(proj.AppliedEvents))
	}

	proj.ExpectApplied(t, storeEvents...)

	if proj.Foo != 0 {
		t.Fatalf("Projection should have been reset")
	}
}
