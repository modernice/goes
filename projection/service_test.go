package projection_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/event/eventbus"
	"github.com/modernice/goes/event/eventstore"
	"github.com/modernice/goes/internal/projectiontest"
	"github.com/modernice/goes/projection"
	"github.com/modernice/goes/projection/schedule"
)

func TestService_Trigger_unregisteredName(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus := eventbus.New[uuid.UUID]()

	svc := projection.NewService[uuid.UUID](uuid.New, bus, projection.TriggerTimeout[uuid.UUID](time.Second))

	if err := svc.Trigger(ctx, "example"); !errors.Is(err, projection.ErrUnhandledTrigger) {
		t.Fatalf("Trigger should fail with %q when passing an unregistered name; got %q", projection.ErrUnhandledTrigger, err)
	}
}

func TestService_Trigger_serviceNotRunning(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus := eventbus.New[uuid.UUID]()
	store := eventstore.New[uuid.UUID]()

	s := schedule.Continuously(bus, store, []string{"foo", "bar", "baz"})

	projection.NewService[uuid.UUID](uuid.New, bus, projection.RegisterSchedule[uuid.UUID]("example", s))
	svc := projection.NewService[uuid.UUID](uuid.New, bus, projection.TriggerTimeout[uuid.UUID](time.Second))

	if err := svc.Trigger(ctx, "example"); !errors.Is(err, projection.ErrUnhandledTrigger) {
		t.Fatalf("Trigger should fail with %q when the handler Service is not running; got %q", projection.ErrUnhandledTrigger, err)
	}
}

func TestService_Trigger(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus := eventbus.New[uuid.UUID]()
	store, storeEvents := newEventStore(t)

	s := schedule.Continuously(bus, store, []string{"foo", "bar", "baz"})
	proj := projectiontest.NewMockProjection()
	applied := make(chan struct{})

	errs, err := s.Subscribe(ctx, func(job projection.Job[uuid.UUID]) error {
		defer close(applied)
		return job.Apply(job, proj)
	})
	if err != nil {
		t.Fatalf("subscribe to schedule: %v", err)
	}

	handler := projection.NewService[uuid.UUID](uuid.New, bus, projection.RegisterSchedule[uuid.UUID]("example", s))
	handlerErrors, err := handler.Run(ctx)
	if err != nil {
		t.Fatalf("Run failed with %q", err)
	}

	svc := projection.NewService(uuid.New, bus)

	if err := svc.Trigger(ctx, "example"); err != nil {
		t.Fatalf("Trigger failed with %q", err)
	}

	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
L:
	for {
		select {
		case <-timer.C:
			t.Fatal("timed out")
		case err := <-errs:
			t.Fatal(err)
		case err := <-handlerErrors:
			t.Fatal(err)
		case <-applied:
			break L
		}
	}

	proj.ExpectApplied(t, storeEvents...)
}

func TestService_Trigger_TriggerOption(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus := eventbus.New[uuid.UUID]()
	store, storeEvents := newEventStore(t)

	s := schedule.Continuously(bus, store, []string{"foo", "bar", "baz"})
	proj := projectiontest.NewMockResetProjection(8)
	proj.SetProgress(time.Now())
	applied := make(chan struct{})

	errs, err := s.Subscribe(ctx, func(job projection.Job[uuid.UUID]) error {
		defer close(applied)
		return job.Apply(job, proj)
	})
	if err != nil {
		t.Fatalf("subscribe to schedule: %v", err)
	}

	handler := projection.NewService[uuid.UUID](uuid.New, bus, projection.RegisterSchedule[uuid.UUID]("example", s))
	handlerErrors, err := handler.Run(ctx)
	if err != nil {
		t.Fatalf("Run failed with %q", err)
	}

	svc := projection.NewService(uuid.New, bus)

	if err := svc.Trigger(ctx, "example", projection.Reset[uuid.UUID](true)); err != nil {
		t.Fatalf("Trigger failed with %q", err)
	}

	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
L:
	for {
		select {
		case <-timer.C:
			t.Fatal("timed out")
		case err := <-errs:
			t.Fatal(err)
		case err := <-handlerErrors:
			t.Fatal(err)
		case <-applied:
			break L
		}
	}

	proj.ExpectApplied(t, storeEvents...)

	if proj.Foo != 0 {
		t.Fatalf("Projection should have been reset")
	}
}
