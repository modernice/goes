package schedule_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventstore"
	"github.com/modernice/goes/event/query"
	"github.com/modernice/goes/event/test"
	"github.com/modernice/goes/helper/streams"
	"github.com/modernice/goes/internal"
	"github.com/modernice/goes/internal/projectiontest"
	"github.com/modernice/goes/projection"
	"github.com/modernice/goes/projection/schedule"
)

func TestPeriodic_Subscribe(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store := eventstore.New()

	events := []event.Event{
		event.New[any]("foo", test.FooEventData{}),
		event.New[any]("bar", test.FooEventData{}),
		event.New[any]("baz", test.FooEventData{}),
		event.New[any]("foobar", test.FooEventData{}),
	}

	if err := store.Insert(ctx, events...); err != nil {
		t.Fatalf("insert events: %v", err)
	}

	schedule := schedule.Periodically(store, 20*time.Millisecond, []string{"foo", "bar", "baz"})

	proj := projectiontest.NewMockProjection()

	subscribeCtx, cancelSubscribe := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancelSubscribe()

	appliedJobs := make(chan projection.Job)

	errs, err := schedule.Subscribe(subscribeCtx, func(job projection.Job) error {
		if err := job.Apply(context.Background(), proj); err != nil {
			return fmt.Errorf("apply Job: %w", err)
		}
		appliedJobs <- job
		return nil
	})
	if err != nil {
		t.Fatalf("Subscribe failed with %q", err)
	}

	var applyCount int
	timeout := time.NewTimer(2 * time.Second)
	defer timeout.Stop()
L:
	for {
		select {
		case <-timeout.C:
			t.Fatal("timed out")
		case err, ok := <-errs:
			if !ok {
				break L
			}
			t.Fatal(err)
		case <-appliedJobs:
			applyCount++
		}
	}

	if applyCount < 8 || applyCount > 12 {
		t.Fatalf("~%d Jobs should have been created; got %d", 10, applyCount)
	}
}

func TestPeriodic_Subscribe_Startup(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store := eventstore.New()

	storeEvents := []event.Event{
		event.New("foo", test.FooEventData{}, event.Aggregate(internal.NewUUID(), "foo", 1)).Any(),
		event.New("bar", test.BarEventData{}, event.Aggregate(internal.NewUUID(), "bar", 1)).Any(),
		event.New("baz", test.BazEventData{}, event.Aggregate(internal.NewUUID(), "baz", 1)).Any(),
		event.New("foobar", test.FoobarEventData{}, event.Aggregate(internal.NewUUID(), "foobar", 1)).Any(),
	}

	if err := store.Insert(ctx, storeEvents...); err != nil {
		t.Fatalf("insert events: %v", err)
	}

	sch := schedule.Periodically(store, 500*time.Millisecond, []string{"foo", "bar", "baz", "foobar"})

	queriedRefs := make(chan []aggregate.Ref, 1)

	subCtx, cancelSubscription := context.WithCancel(ctx)

	errs, err := sch.Subscribe(subCtx, func(ctx projection.Job) error {
		defer cancelSubscription()

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
