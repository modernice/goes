package schedule_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventbus"
	"github.com/modernice/goes/event/eventstore"
	"github.com/modernice/goes/event/query"
	"github.com/modernice/goes/event/test"
	"github.com/modernice/goes/helper/streams"
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
		event.New[any]("foo", test.FooEventData{}),
		event.New[any]("bar", test.FooEventData{}),
		event.New[any]("baz", test.FooEventData{}),
		event.New[any]("foobar", test.FooEventData{}),
	}

	if err := bus.Publish(ctx, events...); err != nil {
		t.Fatalf("publish events: %v", err)
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
		event.New[any]("foo", test.FooEventData{}),
		event.New[any]("bar", test.FooEventData{}),
		event.New[any]("baz", test.FooEventData{}),
		event.New[any]("foobar", test.FooEventData{}),
	}

	if err := bus.Publish(ctx, events...); err != nil {
		t.Fatalf("publish events: %v", err)
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
		event.New[any]("foo", test.FooEventData{}, event.Time(now.Add(-time.Minute))),
		event.New[any]("bar", test.BarEventData{}, event.Time(now)),
		event.New[any]("baz", test.BazEventData{}, event.Time(now.Add(time.Second))),
		event.New[any]("foobar", test.FoobarEventData{}, event.Time(now.Add(time.Minute))),
	}

	if err := bus.Publish(ctx, events...); err != nil {
		t.Fatalf("publish events: %v", err)
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

	test.AssertEqualEvents(t, proj.AppliedEvents, events[1:3])
}

func TestContinuous_Trigger(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus := eventbus.New()
	store := eventstore.New()

	storeEvents := []event.Event{
		event.New[any]("foo", test.FooEventData{}),
		event.New[any]("bar", test.FooEventData{}),
		event.New[any]("baz", test.FooEventData{}),
		event.New[any]("foobar", test.FooEventData{}),
	}

	if err := store.Insert(ctx, storeEvents...); err != nil {
		t.Fatalf("insert events: %v", err)
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
		event.New[any]("foo", test.FooEventData{}),
		event.New[any]("bar", test.FooEventData{}),
		event.New[any]("baz", test.FooEventData{}),
		event.New[any]("foobar", test.FooEventData{}),
	}

	if err := store.Insert(ctx, storeEvents...); err != nil {
		t.Fatalf("insert events: %v", err)
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
		t.Fatalf("%d events should be applied; got %d", 2, len(proj.AppliedEvents))
	}

	proj.ExpectApplied(t, storeEvents[1], storeEvents[3])
}

func TestContinuous_Trigger_Query(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus := eventbus.New()
	store := eventstore.New()

	storeEvents := []event.Event{
		event.New[any]("foo", test.FooEventData{}),
		event.New[any]("bar", test.FooEventData{}),
		event.New[any]("baz", test.FooEventData{}),
		event.New[any]("foobar", test.FooEventData{}),
	}

	if err := store.Insert(ctx, storeEvents...); err != nil {
		t.Fatalf("insert events: %v", err)
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
		event.New[any]("foo", test.FooEventData{}, event.Time(now)),
		event.New[any]("bar", test.FooEventData{}, event.Time(now.Add(time.Second))),
		event.New[any]("baz", test.FooEventData{}, event.Time(now.Add(time.Minute))),
		event.New[any]("foobar", test.FooEventData{}, event.Time(now.Add(time.Hour))),
	}

	if err := store.Insert(ctx, storeEvents...); err != nil {
		t.Fatalf("insert events: %v", err)
	}

	sch := schedule.Continuously(bus, store, []string{"foo", "bar", "baz", "foobar"})

	proj := projectiontest.NewMockResetProjection(7)

	projection.Apply(proj, storeEvents)

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
		if err := sch.Trigger(ctx, projection.Reset(true)); err != nil {
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

func TestContinuous_Subscribe_Startup(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus := eventbus.New()
	store := eventstore.New()

	storeEvents := []event.Event{
		event.New("foo", test.FooEventData{}, event.Aggregate(uuid.New(), "foo", 1)).Any(),
		event.New("bar", test.BarEventData{}, event.Aggregate(uuid.New(), "bar", 1)).Any(),
		event.New("baz", test.BazEventData{}, event.Aggregate(uuid.New(), "baz", 1)).Any(),
		event.New("foobar", test.FoobarEventData{}, event.Aggregate(uuid.New(), "foobar", 1)).Any(),
	}

	if err := store.Insert(ctx, storeEvents...); err != nil {
		t.Fatalf("insert events: %v", err)
	}

	sch := schedule.Continuously(bus, store, []string{"foo", "bar", "baz", "foobar"})

	queriedRefs := make(chan []aggregate.Ref)

	errs, err := sch.Subscribe(ctx, func(ctx projection.Job) error {
		str, errs, err := ctx.Aggregates(ctx)
		if err != nil {
			return err
		}
		refs, err := streams.Drain(ctx, str, errs)
		if err != nil {
			return err
		}
		queriedRefs <- refs
		return nil
	}, projection.Startup(projection.Query(query.New(query.Name("foo", "baz")))))
	if err != nil {
		t.Fatalf("Subscribe() failed with %q", err)
	}

	var refs []aggregate.Ref
	select {
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out. startup projection job not triggered?")
	case err := <-errs:
		t.Fatal(err)
	case refs = <-queriedRefs:
	}

	if len(refs) != 2 {
		t.Fatalf("projection job should return 2 aggregates; got %d", len(refs))
	}

L:
	for _, name := range []string{"foo", "baz"} {
		for _, ref := range refs {
			if ref.Name == name {
				continue L
			}
		}

		t.Fatalf("projection job should return aggregate %q", name)
	}
}

func TestContinuous_Subscribe_BeforeEvent(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus := eventbus.New()
	store := eventstore.New()

	mainEvents := []event.Event{
		event.New("foo", test.FooEventData{}).Any(),
		event.New("bar", test.BarEventData{}).Any(),
		event.New("baz", test.BazEventData{}).Any(),
	}

	addEvents := []event.Event{
		event.New("foobar", test.FoobarEventData{}).Any(),
		event.New("bar", test.BarEventData{}).Any(),
	}

	sch := schedule.Continuously(bus, store, []string{"foo", "bar", "baz"}, schedule.Debounce(200*time.Millisecond))

	queriedEvents := make(chan []event.Event)
	errs, err := sch.Subscribe(ctx, func(ctx projection.Job) error {
		str, errs, err := ctx.Events(ctx)
		events, err := streams.Drain(ctx, str, errs)
		if err != nil {
			return err
		}
		queriedEvents <- events
		return nil
	}, projection.BeforeEvent(func(context.Context, event.Event) ([]event.Event, error) {
		return addEvents, nil
	}, "foo", "baz"))
	if err != nil {
		t.Fatalf("Subscribe() failed with %q", err)
	}

	if err := bus.Publish(ctx, mainEvents...); err != nil {
		t.Fatalf("publish events: %v", err)
	}

	var events []event.Event
	select {
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out")
	case err := <-errs:
		t.Fatal(err)
	case events = <-queriedEvents:
	}

	want := append(append(addEvents, mainEvents[:2]...), append(addEvents, mainEvents[2])...)

	if len(events) != len(want) {
		t.Fatalf("projection job should return %d events; got %d", len(want), len(events))
	}

	if !cmp.Equal(want, events) {
		t.Fatalf("projection job returned wrong events\n%s", cmp.Diff(want, events))
	}
}
