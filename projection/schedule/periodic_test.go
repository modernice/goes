package schedule_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventstore"
	"github.com/modernice/goes/event/test"
	"github.com/modernice/goes/internal/projectiontest"
	"github.com/modernice/goes/projection"
	"github.com/modernice/goes/projection/schedule"
)

func TestPeriodic_Subscribe(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store := eventstore.New[any]()

	events := []event.EventOf[any]{
		event.New[any]("foo", test.FooEventData{}),
		event.New[any]("bar", test.FooEventData{}),
		event.New[any]("baz", test.FooEventData{}),
		event.New[any]("foobar", test.FooEventData{}),
	}

	if err := store.Insert(ctx, events...); err != nil {
		t.Fatalf("insert Events: %v", err)
	}

	schedule := schedule.Periodically(store, 20*time.Millisecond, []string{"foo", "bar", "baz"})

	proj := projectiontest.NewMockProjection()

	subscribeCtx, cancelSubscribe := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancelSubscribe()

	appliedJobs := make(chan projection.Job[any])

	errs, err := schedule.Subscribe(subscribeCtx, func(job projection.Job[any]) error {
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
