package project_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventbus/chanbus"
	"github.com/modernice/goes/event/eventstore/memstore"
	"github.com/modernice/goes/event/test"
	"github.com/modernice/goes/project"
)

func TestService_Trigger(t *testing.T) {
	reg := event.NewRegistry()
	ebus := chanbus.New()
	estore := memstore.New()

	eventNames := []string{"foo", "bar", "baz"}
	schedule := project.Continuously(ebus, estore, eventNames)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	triggered := make(chan struct{})

	subscriptionErrors, err := schedule.Subscribe(ctx, func(project.Job) error {
		close(triggered)
		return nil
	})
	if err != nil {
		t.Fatalf("subscribe to Schedule: %v", err)
	}

	svc := project.NewService(reg, ebus, project.RegisterSchedule("example", schedule))

	errs, err := svc.Run(ctx)
	if err != nil {
		t.Fatalf("Run failed with %q", err)
	}

	if err := svc.Trigger(ctx, "example"); err != nil {
		t.Fatalf("Trigger failed with %q", err)
	}

	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			t.Fatal("timed out")
		case err, ok := <-subscriptionErrors:
			if ok {
				t.Fatal(err)
			}
		case err, ok := <-errs:
			if ok {
				t.Fatal(err)
			}
		case <-triggered:
			return
		}
	}
}

func TestService_Trigger_Fully(t *testing.T) {
	reg := event.NewRegistry()
	ebus := chanbus.New()
	estore := memstore.New()

	eventNames := []string{"foo", "bar", "baz"}
	schedule := project.Continuously(ebus, estore, eventNames)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	triggered := make(chan struct{})
	mockProjection := newProgressed()
	mockProjection.Progressor.LatestEventTime = time.Now().UnixNano()

	events := []event.Event{
		event.New("foo", test.FooEventData{}, event.Time(time.Now().Add(-time.Hour))),
		event.New("bar", test.FooEventData{}, event.Time(time.Now())),
		event.New("baz", test.FooEventData{}, event.Time(time.Now().Add(time.Minute))),
	}

	if err := estore.Insert(ctx, events...); err != nil {
		t.Fatalf("insert events: %v", err)
	}

	subscriptionErrors, err := schedule.Subscribe(ctx, func(job project.Job) error {
		if err := job.Apply(job.Context(), mockProjection); err != nil {
			return fmt.Errorf("apply Job: %v", err)
		}
		close(triggered)
		return nil
	})
	if err != nil {
		t.Fatalf("subscribe to Schedule: %v", err)
	}

	svc := project.NewService(reg, ebus, project.RegisterSchedule("example", schedule))

	errs, err := svc.Run(ctx)
	if err != nil {
		t.Fatalf("Run failed with %q", err)
	}

	if err := svc.Trigger(ctx, "example", project.Fully()); err != nil {
		t.Fatalf("Trigger failed with %q", err)
	}

	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
L:
	for {
		select {
		case <-timer.C:
			t.Fatal("timed out")
		case err, ok := <-subscriptionErrors:
			if ok {
				t.Fatal(err)
			}
		case err, ok := <-errs:
			if ok {
				t.Fatal(err)
			}
		case <-triggered:
			break L
		}
	}

	if !mockProjection.hasApplied(events...) {
		t.Fatalf("%v events should have been applied", events)
	}
}

func TestService_Trigger_noHandler(t *testing.T) {
	reg := event.NewRegistry()
	ebus := chanbus.New()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	svc := project.NewService(reg, ebus)

	if err := svc.Trigger(ctx, "example", project.TriggerTimeout(time.Second)); !errors.Is(err, project.ErrUnhandledTrigger) {
		t.Fatalf("Trigger should fail with %q when no Service handles a triggered Schedule; got %q", project.ErrUnhandledTrigger, err)
	}
}
