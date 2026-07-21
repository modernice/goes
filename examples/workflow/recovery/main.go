// Command recovery demonstrates that workflows survive restarts:
//
//   - a trigger event that was persisted while no service was running is
//     picked up by the startup trigger replay (Config.TriggerReplayWindow)
//   - the first service instance is stopped before a scheduled timeout
//     fires; nothing happens while no instance runs
//   - a fresh instance recovers the overdue timeout from the event store and
//     fires it immediately
//   - the command that the first instance already dispatched is NOT
//     dispatched again, because the dispatch was recorded on the workflow
package main

import (
	"context"
	"errors"
	"log"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate/repository"
	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/command"
	"github.com/modernice/goes/command/cmdbus"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventbus"
	"github.com/modernice/goes/event/eventstore"
	"github.com/modernice/goes/workflow"
)

const (
	OrderAggregate = "shop.order"
	OrderPlaced    = "shop.order.placed"
	ReserveStock   = "shop.reserve_stock"

	OrderWorkflowName = "shop.order_workflow"

	paymentDeadline = time.Second
)

type OrderPlacedData struct{}

type ReservePayload struct{ OrderID uuid.UUID }

type OrderWorkflow struct {
	*workflow.Base
}

func NewOrderWorkflow(id uuid.UUID) *OrderWorkflow {
	return &OrderWorkflow{Base: workflow.New(OrderWorkflowName, id)}
}

var OrderProcess = workflow.Define(
	NewOrderWorkflow,
	workflow.Starts(workflow.ByAggregateID, (*OrderWorkflow).onPlaced, OrderPlaced),
	workflow.OnTimeout("payment-due", (*OrderWorkflow).onPaymentDue),
)

func (w *OrderWorkflow) onPlaced(ctx workflow.Ctx[OrderPlacedData]) error {
	orderID, _, _ := ctx.Event().Aggregate()
	log.Printf("[workflow] order placed; reserving stock and starting the payment clock")

	if err := ctx.Dispatch("reserve", command.New(
		ReserveStock, ReservePayload{OrderID: orderID}, command.Aggregate(OrderAggregate, orderID),
	).Any()); err != nil {
		return err
	}

	return ctx.Schedule("payment-due", time.Now().Add(paymentDeadline))
}

func (w *OrderWorkflow) onPaymentDue(ctx workflow.Ctx[workflow.TimeoutFiredData]) error {
	log.Printf("[workflow] payment deadline passed (was due %s ago)", time.Since(ctx.Event().Data().ScheduledFor).Round(time.Millisecond))
	return ctx.Fail(errors.New("payment not received in time"))
}

func main() {
	log.SetFlags(0)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reg := codec.New()
	codec.Register[OrderPlacedData](reg, OrderPlaced)
	codec.Register[ReservePayload](reg, ReserveStock)

	// The store outlives the service instances.
	store := eventstore.New()

	cmdBus := cmdbus.New[int](reg, eventbus.New())
	cmdBusErrs, err := cmdBus.Run(ctx)
	must(err)
	go drain("command bus", cmdBusErrs)

	var reservations atomic.Int32
	go drain("reserve handler", command.MustHandle(ctx, cmdBus, ReserveStock, func(cmd command.Ctx[ReservePayload]) error {
		reservations.Add(1)
		log.Printf("[stock   ] reservation #%d for order %s", reservations.Load(), cmd.Payload().OrderID)
		return nil
	}))

	repo := repository.New(store)
	orderID := uuid.New()

	// The order is persisted while NO workflow service is running — e.g.
	// another process wrote it while this one was down.
	log.Printf("--- no service running ---")
	log.Printf("[shop    ] persisting order %s", orderID)
	must(store.Insert(ctx, event.New(
		OrderPlaced, OrderPlacedData{}, event.Aggregate(orderID, OrderAggregate, 1),
	).Any()))

	// Instance #1 replays the trigger from the store on startup, starts the
	// workflow, and dispatches the reserve command — then "crashes" before
	// the payment deadline fires.
	log.Printf("--- instance #1 starts ---")
	ctx1, stop1 := context.WithCancel(ctx)
	svc1 := newService(reg, store, cmdBus)
	errs1, err := svc1.Run(ctx1)
	must(err)
	go drain("instance #1", errs1)

	await("instance #1 to dispatch the reserve command", func() bool {
		return reservations.Load() == 1
	})

	log.Printf("--- instance #1 crashes ---")
	stop1()

	// While no instance runs, the deadline passes without consequence.
	time.Sleep(paymentDeadline + 200*time.Millisecond)
	if w := fetch(repo, orderID); w.Status() != workflow.StatusRunning {
		log.Fatalf("expected workflow to still be running; got %s", w.Status())
	}
	log.Printf("deadline passed while offline; workflow still %q", fetch(repo, orderID).Status())

	// Instance #2 recovers the overdue timeout from the store and fires it.
	// The startup trigger replay sees the order again, but the recorded
	// trigger deduplicates it, and the already dispatched command is not
	// dispatched again.
	log.Printf("--- instance #2 starts ---")
	svc2 := newService(reg, store, cmdBus)
	errs2, err := svc2.Run(ctx)
	must(err)
	go drain("instance #2", errs2)

	await("instance #2 to fire the overdue deadline", func() bool {
		return fetch(repo, orderID).Status() == workflow.StatusFailed
	})

	w := fetch(repo, orderID)
	log.Printf("final status: %s (reason %q)", w.Status(), w.Reason())
	log.Printf("stock reservations: %d (no duplicate dispatch)", reservations.Load())
}

func newService(reg codec.Encoding, store event.Store, cmdBus command.Bus) *workflow.Service {
	return workflow.NewService(workflow.Config{
		Commands:         reg,
		EventStore:       store,
		EventBus:         eventbus.New(),
		CommandBus:       cmdBus,
		DispatchInterval: 25 * time.Millisecond,
		TimerResolution:  10 * time.Millisecond,

		// Trigger replay is opt-in: re-process triggers of the last 24h
		// that this instance may have missed while it was down.
		TriggerReplayWindow: 24 * time.Hour,
	}, OrderProcess)
}

func fetch(repo *repository.Repository, id uuid.UUID) *OrderWorkflow {
	w := NewOrderWorkflow(id)
	must(repo.Fetch(context.Background(), w))
	return w
}

func await(what string, cond func() bool) {
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	log.Fatalf("timed out waiting for %s", what)
}

func drain(name string, errs <-chan error) {
	for err := range errs {
		if err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("[%s] error: %v", name, err)
		}
	}
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
