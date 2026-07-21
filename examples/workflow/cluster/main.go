// Command cluster demonstrates how multiple service instances cooperate
// through a shared event store:
//
//   - effects recorded by one instance are discovered and executed by
//     another instance through periodic resyncs (Config.ResyncInterval)
//   - an instance with a stale in-memory view never fires a timeout that
//     another instance already canceled: before firing, the workflow is
//     fetched from the store and must still hold the active timeout
//
// Instance A stands in for a separate process that shares the event store
// but not the event bus; its trigger events are fed in directly through
// Service.Trigger for brevity.
package main

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate/repository"
	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/command"
	"github.com/modernice/goes/command/cmdbus"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventbus"
	"github.com/modernice/goes/event/eventstore"
	"github.com/modernice/goes/event/query"
	"github.com/modernice/goes/helper/streams"
	"github.com/modernice/goes/workflow"
)

const (
	OrderAggregate = "shop.order"

	OrderPlaced     = "shop.order.placed"
	PaymentReceived = "shop.order.payment_received"

	NotifyWarehouse = "shop.notify_warehouse"

	OrderWorkflowName = "shop.order_workflow"

	paymentDeadline = 900 * time.Millisecond
)

type OrderPlacedData struct{}
type PaymentReceivedData struct{}

type NotifyPayload struct{ OrderID uuid.UUID }

// OrderWorkflow notifies the warehouse about new orders and watches the
// payment deadline. Receiving the payment cancels the deadline but keeps the
// workflow running (shipping would follow).
type OrderWorkflow struct {
	*workflow.Base
}

func NewOrderWorkflow(id uuid.UUID) *OrderWorkflow {
	return &OrderWorkflow{Base: workflow.New(OrderWorkflowName, id)}
}

var OrderProcess = workflow.Define(
	NewOrderWorkflow,
	workflow.Starts(workflow.ByAggregateID, (*OrderWorkflow).onPlaced, OrderPlaced),
	workflow.Reacts(workflow.ByAggregateID, (*OrderWorkflow).onPayment, PaymentReceived),
	workflow.OnTimeout("payment-due", (*OrderWorkflow).onPaymentDue),
)

func (w *OrderWorkflow) onPlaced(ctx workflow.Ctx[OrderPlacedData]) error {
	orderID, _, _ := ctx.Event().Aggregate()

	if err := ctx.Dispatch("notify", command.New(
		NotifyWarehouse, NotifyPayload{OrderID: orderID}, command.Aggregate(OrderAggregate, orderID),
	).Any()); err != nil {
		return err
	}

	return ctx.Schedule("payment-due", time.Now().Add(paymentDeadline))
}

func (w *OrderWorkflow) onPayment(ctx workflow.Ctx[PaymentReceivedData]) error {
	orderID, _, _ := ctx.Event().Aggregate()
	log.Printf("[inst A  ] order %s: payment received; canceling the deadline", short(orderID))
	return ctx.Unschedule("payment-due")
}

func (w *OrderWorkflow) onPaymentDue(ctx workflow.Ctx[workflow.TimeoutFiredData]) error {
	log.Printf("[workflow] ⚠️ order %s: payment deadline fired", short(ctx.Event().Data().WorkflowID))
	return nil
}

func main() {
	log.SetFlags(0)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reg := codec.New()
	codec.Register[OrderPlacedData](reg, OrderPlaced)
	codec.Register[PaymentReceivedData](reg, PaymentReceived)
	codec.Register[NotifyPayload](reg, NotifyWarehouse)

	// --- Part 1: effects travel between instances through the store. ---
	//
	// Instance A records a command effect. Instance B — running with its own
	// event bus, so it never sees A's triggers or notifications — discovers
	// the pending command through its periodic resync and dispatches it.
	log.Printf("--- part 1: resync shares effects between instances ---")

	store := eventstore.New()

	cmdBus := cmdbus.New[int](reg, eventbus.New())
	cmdBusErrs, err := cmdBus.Run(ctx)
	must(err)
	go drain("command bus", cmdBusErrs)

	notified := make(chan uuid.UUID, 1)
	go drain("notify handler", command.MustHandle(ctx, cmdBus, NotifyWarehouse, func(cmd command.Ctx[NotifyPayload]) error {
		log.Printf("[whse    ] notified about order %s", cmd.Payload().OrderID)
		select {
		case notified <- cmd.Payload().OrderID:
		default:
		}
		return nil
	}))

	instanceA := workflow.NewService(workflow.Config{
		Commands:   reg,
		EventStore: store,
	}, OrderProcess)

	instanceB := workflow.NewService(workflow.Config{
		Commands:         reg,
		EventStore:       store,
		EventBus:         eventbus.New(),
		CommandBus:       cmdBus,
		DispatchInterval: 25 * time.Millisecond,
		TimerResolution:  10 * time.Millisecond,
		ResyncInterval:   150 * time.Millisecond,
	}, OrderProcess)

	errsB, err := instanceB.Run(ctx)
	must(err)
	go drain("instance B", errsB)

	order1 := uuid.New()
	log.Printf("[inst A  ] handling order %s (records the notify command)", order1)
	must(instanceA.Trigger(ctx, event.New(
		OrderPlaced, OrderPlacedData{}, event.Aggregate(order1, OrderAggregate, 1),
	).Any()))

	select {
	case <-notified:
		log.Printf("instance B dispatched the command that instance A recorded")
	case <-time.After(5 * time.Second):
		log.Fatal("timed out waiting for instance B to dispatch the command")
	}

	// Conclude order 1 so its payment deadline does not fire later on.
	must(instanceA.Trigger(ctx, event.New(
		PaymentReceived, PaymentReceivedData{}, event.Aggregate(order1, OrderAggregate, 2),
	).Any()))

	// --- Part 2: stale instances cannot fire canceled timeouts. ---
	//
	// Instance C loads the payment deadline into memory, then instance A
	// cancels it. C never learns about the cancellation (its periodic resync
	// is disabled), but the fire is validated against the store — so the
	// canceled deadline does not fire.
	log.Printf("--- part 2: canceled timeouts never fire on stale instances ---")

	store2 := eventstore.New()

	instanceA2 := workflow.NewService(workflow.Config{
		Commands:   reg,
		EventStore: store2,
	}, OrderProcess)

	instanceC := workflow.NewService(workflow.Config{
		Commands:         reg,
		EventStore:       store2,
		EventBus:         eventbus.New(),
		CommandBus:       cmdBus,
		DispatchInterval: 25 * time.Millisecond,
		TimerResolution:  10 * time.Millisecond,
		ResyncInterval:   -1, // Only the initial recovery on startup.
	}, OrderProcess)

	order2 := uuid.New()
	log.Printf("[inst A  ] handling order %s (schedules the payment deadline)", order2)
	must(instanceA2.Trigger(ctx, event.New(
		OrderPlaced, OrderPlacedData{}, event.Aggregate(order2, OrderAggregate, 1),
	).Any()))

	errsC, err := instanceC.Run(ctx)
	must(err)
	go drain("instance C", errsC)

	// Once C dispatched the notify command, it has recovered order2's state
	// — including the payment deadline — into memory.
	select {
	case <-notified:
	case <-time.After(5 * time.Second):
		log.Fatal("timed out waiting for instance C to recover the pending effects")
	}
	log.Printf("[inst C  ] recovered order state, deadline armed in memory")

	must(instanceA2.Trigger(ctx, event.New(
		PaymentReceived, PaymentReceivedData{}, event.Aggregate(order2, OrderAggregate, 2),
	).Any()))

	// Let the deadline pass. Instance C attempts to fire it, but the fetched
	// workflow no longer holds the timeout, so the fire is dropped.
	time.Sleep(paymentDeadline + 300*time.Millisecond)

	repo := repository.New(store2)
	w := NewOrderWorkflow(order2)
	must(repo.Fetch(context.Background(), w))

	if fired := countWorkflowEvents(store2, order2, workflow.TimeoutFired); fired == 0 {
		log.Printf("deadline passed on stale instance C: 0 timeouts fired, workflow still %q ✔", w.Status())
	} else {
		log.Fatalf("canceled timeout fired %d time(s)", fired)
	}
}

func countWorkflowEvents(store event.Store, workflowID uuid.UUID, name string) int {
	events, errs, err := store.Query(context.Background(), query.New(
		query.Name(name),
		query.AggregateName(OrderWorkflowName),
		query.AggregateID(workflowID),
	))
	must(err)

	all, err := streams.Drain(context.Background(), events, errs)
	must(err)

	return len(all)
}

func short(id uuid.UUID) string {
	return id.String()[:8]
}

func drain(name string, errs <-chan error) {
	for err := range errs {
		if err != nil {
			log.Printf("[%s] error: %v", name, err)
		}
	}
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
