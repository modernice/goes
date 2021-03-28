package project_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate/project"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventbus/chanbus"
	"github.com/modernice/goes/event/eventstore/memstore"
	"github.com/modernice/goes/event/query"
	"github.com/modernice/goes/event/query/version"
	"github.com/modernice/goes/event/test"
)

func TestContinuously(t *testing.T) {
	store, bus := newStoreBus()
	proj := project.NewProjector(store)

	sub, errs, err := project.Subscribe(
		context.Background(),
		project.Continuously(bus, []string{"foo"}),
		proj,
	)
	if err != nil {
		t.Fatalf("Subscribe shouldn't fail; failed with %q", err)
	}

	events := append(makeEvents(3), event.New("foo", test.FooEventData{}))

	pubError := make(chan error)
	go func() {
		for _, evt := range events {
			if err = bus.Publish(context.Background(), evt); err != nil {
				pubError <- fmt.Errorf("failed to publish Events: %w", err)
			}
			// Add artificial delay to guarantee correct order of incoming Events
			<-time.After(5 * time.Millisecond)
		}
	}()

	for _, evt := range events[:3] {
		select {
		case <-time.After(10 * time.Millisecond):
			t.Fatalf("didn't receive Event after %s", 10*time.Millisecond)
		case err, ok := <-errs:
			if !ok {
				errs = nil
				break
			}
			t.Fatal(err)
		case err := <-pubError:
			t.Fatal(err)
		case ctx := <-sub:
			if ctx.AggregateName() != evt.AggregateName() {
				t.Errorf("Projection has wrong AggregateName. want=%q got=%q", evt.AggregateName(), ctx.AggregateName())
			}

			if ctx.AggregateID() != evt.AggregateID() {
				t.Errorf("Projection has wrong AggregateID. want=%s got=%s", evt.AggregateID(), ctx.AggregateID())
			}
		}
	}

	select {
	case <-sub:
		t.Errorf("shouldn't receive Context that doesn't belong to an Aggregate!")
	case <-time.After(5 * time.Millisecond):
	}
}

func TestContinuously_Project(t *testing.T) {
	store, bus := newStoreBus()
	proj := project.NewProjector(store)

	sub, _, err := project.Subscribe(
		context.Background(),
		project.Continuously(bus, []string{"foo"}),
		proj,
	)

	if err != nil {
		t.Fatalf("Subscribe shouldn't fail; failed with %q", err)
	}

	evt := event.New("foo", test.FooEventData{}, event.Aggregate("foo", uuid.New(), 0))
	pubError := make(chan error)
	go func() {
		if err := bus.Publish(context.Background(), evt); err != nil {
			pubError <- fmt.Errorf("failed to publish Event: %v", err)
		}
	}()

	var ctx project.Context
	select {
	case <-time.After(10 * time.Millisecond):
		t.Fatalf("didn't receive Context after %s", 10*time.Millisecond)
	case err := <-pubError:
		t.Fatal(err)
	case ctx = <-sub:
	}

	if ctx.AggregateName() != "foo" {
		t.Errorf("Context has wrong AggregateName. want=%q got=%q", "foo", ctx.AggregateName())
	}

	if ctx.AggregateID() != evt.AggregateID() {
		t.Errorf("Context has wrong AggregateID. want=%s got=%s", evt.AggregateID(), ctx.AggregateID())
	}

	events := make([]event.Event, 10)
	events[0] = evt
	for i := range events[1:] {
		events[i+1] = event.New("foo", test.FooEventData{}, event.Previous(events[i]))
	}
	if err := store.Insert(context.Background(), events...); err != nil {
		t.Fatalf("failed to insert Events: %v", err)
	}

	mp := newMockProjection(evt.AggregateID())

	if err = ctx.Project(ctx, mp); err != nil {
		t.Fatalf("projection shouldn't fail; failed with %q", err)
	}

	test.AssertEqualEvents(t, mp.events, events)

	if mp.AggregateVersion() != 9 {
		t.Errorf("Projections should be at version %d; is at version %d", 9, mp.AggregateVersion())
	}
}

func TestPeriodically(t *testing.T) {
	store, _ := newStoreBus()
	proj := project.NewProjector(store)

	events := make([]event.Event, 10)
	for i := range events {
		events[i] = event.New("foo", test.FooEventData{}, event.Aggregate("foo", uuid.New(), 0))
	}
	if err := store.Insert(context.Background(), events...); err != nil {
		t.Fatalf("failed to insert Events: %v", err)
	}

	sub, _, err := project.Subscribe(
		context.Background(),
		project.Periodically(store, 20*time.Millisecond, []string{"foo"}),
		proj,
	)
	if err != nil {
		t.Fatalf("Periodically shouldn't fail; failed with %q", err)
	}

	var prev time.Time
	for i := 0; i < 10; i++ {
		for j := 0; j < 10; j++ {
			select {
			case <-time.After(100 * time.Millisecond):
				t.Fatalf("[%d] didn't receive Context after %s", i, 100*time.Millisecond)
			case <-sub:
			}
		}
		now := time.Now()
		if i != 0 {
			if dur := now.Sub(prev); dur < 10*time.Millisecond || dur > 35*time.Millisecond {
				t.Fatalf("Duration between receives should be ~%s; was %s", 20*time.Millisecond, dur)
			}
		}
		prev = now
	}
}

func TestStopTimeout(t *testing.T) {
	store, bus := newStoreBus()
	proj := project.NewProjector(store)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sub, _, err := project.Subscribe(
		ctx,
		project.Continuously(bus, []string{"foo"}),
		proj,
		project.StopTimeout(50*time.Millisecond),
	)

	if err != nil {
		t.Fatalf("Subscribe shouldn't fail; failed with %q", err)
	}

	events := makeEvents(10)
	pubError := make(chan error)
	go func() {
		if err = bus.Publish(context.Background(), events...); err != nil {
			pubError <- fmt.Errorf("failed to publish Events: %w", err)
		}
	}()

	select {
	case err := <-pubError:
		t.Fatal(err)
	case _, ok := <-sub:
		if !ok {
			t.Fatalf("[0] Context channel shouldn't be closed yet!")
		}
	}

	cancel()

	<-time.After(10 * time.Millisecond)

	select {
	case err := <-pubError:
		t.Fatal(err)
	case _, ok := <-sub:
		if !ok {
			t.Fatalf("[1] Context channel shouldn't be closed yet!")
		}
	}

	<-time.After(50 * time.Millisecond)
	select {
	case err := <-pubError:
		t.Fatal(err)
	case _, ok := <-sub:
		if ok {
			t.Fatalf("[2] Context channel should be closed now!")
		}
	}
}

func TestFilterEvents(t *testing.T) {
	store, bus := newStoreBus()
	proj := project.NewProjector(store)

	sub, _, err := project.Subscribe(
		context.Background(),
		project.Continuously(bus, []string{"foo"}, project.FilterEvents(
			query.AggregateVersion(version.Exact(3)),
		)),
		proj,
	)

	if err != nil {
		t.Fatalf("Subscribe shouldn't fail; failed with %q", err)
	}

	events := makeEvents(10)

	if err := bus.Publish(context.Background(), events...); err != nil {
		t.Errorf("failed to publish Events: %v", err)
	}

	select {
	case <-time.After(50 * time.Millisecond):
		t.Fatalf("didn't receive Context after %s", 50*time.Millisecond)
	case _, ok := <-sub:
		if !ok {
			t.Errorf("should have received a Context")
		}
	}

	select {
	case <-sub:
		t.Errorf("should have received only 1 Context")
	case <-time.After(50 * time.Millisecond):
	}
}

func newStoreBus() (event.Store, event.Bus) {
	return memstore.New(), chanbus.New()
}

func makeEvents(n int) []event.Event {
	events := []event.Event{event.New("foo", test.FooEventData{}, event.Aggregate("foo", uuid.New(), 0))}
	for i := 0; i < n-1; i++ {
		events = append(
			events,
			event.New("foo", test.FooEventData{}, event.Previous(events[len(events)-1])),
		)
	}
	return events
}
