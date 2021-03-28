package order_test

import (
	"context"
	"ecommerce/order"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/project"
	"github.com/modernice/goes/aggregate/repository"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventbus/chanbus"
	"github.com/modernice/goes/event/eventstore"
	"github.com/modernice/goes/event/eventstore/memstore"
)

func TestTimeline(t *testing.T) {
	id := uuid.New()
	o := order.New(id)
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

	bus := chanbus.New()
	store := eventstore.WithBus(memstore.New(), bus)
	repo := repository.New(store)
	if err := repo.Save(context.Background(), o); err != nil {
		t.Fatalf("failed to save Order: %v", err)
	}

	proj := project.NewProjector(store)
	if err := proj.Project(context.Background(), tl); err != nil {
		t.Fatalf("failed to project: %v", err)
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
