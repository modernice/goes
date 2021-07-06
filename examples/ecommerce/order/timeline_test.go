package order_test

import (
	"context"
	"ecommerce/order"
	"ecommerce/order/memory"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/repository"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventbus/chanbus"
	"github.com/modernice/goes/event/eventstore"
	"github.com/modernice/goes/event/eventstore/memstore"
)

func TestTimeline(t *testing.T) {
	id := uuid.New()
	o := order.New(id)
	timelineRepo := memory.NewTimelineRepository()
	tl := order.NewTimeline(id)

	if _, ok := tl.Duration(); ok {
		t.Fatalf("Duration should return false until the Order is complete/canceled.")
	}

	placedAt := time.Now()
	canceledAt := placedAt.Add(15 * time.Minute)

	aggregate.NextEvent(o, order.Placed, order.PlacedEvent{
		Customer: exampleCustomer,
		Items:    exampleItems,
	}, event.Time(placedAt))

	aggregate.NextEvent(o, order.Canceled, order.CanceledEvent{}, event.Time(canceledAt))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus := chanbus.New()
	store := eventstore.WithBus(memstore.New(), bus)
	repo := repository.New(store)
	if err := repo.Save(ctx, o); err != nil {
		t.Fatalf("failed to save Order: %v", err)
	}

	proj := order.NewTimelineProjector(bus, store, timelineRepo)
	if _, err := proj.Run(ctx); err != nil {
		t.Fatalf("run Projector: %v", err)
	}

	if err := proj.Project(ctx, id); err != nil {
		t.Fatalf("failed to project Timeline: %v", err)
	}

	<-time.After(100 * time.Millisecond)

	tl, err := timelineRepo.Fetch(ctx, id)
	if err != nil {
		t.Fatalf("failed to fetch Timeline: %v", err)
	}

	dur, ok := tl.Duration()
	if !ok {
		t.Fatalf("Duration should return true")
	}

	if dur != canceledAt.Sub(placedAt) {
		t.Fatalf("Duration should return %v; got %v", canceledAt.Sub(placedAt), dur)
	}

	steps := tl.Steps
	want := []order.Step{
		{Start: placedAt, End: canceledAt, Desc: fmt.Sprintf("Bob placed an Order for %d Items at %s.", len(exampleItems), placedAt)},
		{Start: canceledAt, Desc: fmt.Sprintf("Order was canceled at %s.", canceledAt)},
	}

	if !reflect.DeepEqual(steps, want) {
		t.Fatalf("Steps should return %v; got %v", want, steps)
	}
}
