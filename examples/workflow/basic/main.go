// Command basic demonstrates the fundamental workflow lifecycle:
//
//   - a trigger event starts a workflow (workflow.Starts)
//   - the workflow records a command effect (Ctx.Dispatch) that the runtime
//     dispatches over the command bus
//   - the workflow schedules a payment deadline (Ctx.Schedule)
//   - a second trigger advances the workflow (workflow.Reacts), cancels the
//     deadline (Ctx.Unschedule), and completes it (Ctx.Complete)
//
// Triggers are correlated with the built-in workflow.ByAggregateID
// correlator: the workflow instance has the same id as the order aggregate
// that emitted the trigger events.
package main

import (
	"context"
	"errors"
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
	"github.com/modernice/goes/workflow"
)

// Domain events and commands.
const (
	OrderAggregate = "shop.order"

	OrderPlaced     = "shop.order.placed"
	PaymentReceived = "shop.order.payment_received"

	ReserveStock = "shop.reserve_stock"
)

type OrderPlacedData struct {
	ItemCount int
}

type PaymentReceivedData struct{}

type ReserveStockPayload struct {
	OrderID   uuid.UUID
	ItemCount int
}

// OrderWorkflow tracks an order from placement to payment.
type OrderWorkflow struct {
	*workflow.Base
}

const OrderWorkflowName = "shop.order_workflow"

func NewOrderWorkflow(id uuid.UUID) *OrderWorkflow {
	return &OrderWorkflow{Base: workflow.New(OrderWorkflowName, id)}
}

// OrderProcess wires the trigger events to the workflow handlers.
var OrderProcess = workflow.Define(
	NewOrderWorkflow,
	workflow.Starts(workflow.ByAggregateID, (*OrderWorkflow).onPlaced, OrderPlaced),
	workflow.Reacts(workflow.ByAggregateID, (*OrderWorkflow).onPayment, PaymentReceived),
	workflow.OnTimeout("payment-due", (*OrderWorkflow).onPaymentDue),
)

func (w *OrderWorkflow) onPlaced(ctx workflow.Ctx[OrderPlacedData]) error {
	orderID, _, _ := ctx.Event().Aggregate()
	log.Printf("[workflow] order placed; reserving %d item(s)", ctx.Event().Data().ItemCount)

	// Record a command effect. The command is not dispatched here: it is
	// persisted with the workflow and dispatched by the runtime, which
	// retries it until the dispatch is recorded.
	if err := ctx.Dispatch("reserve-stock", command.New(
		ReserveStock,
		ReserveStockPayload{OrderID: orderID, ItemCount: ctx.Event().Data().ItemCount},
		command.Aggregate(OrderAggregate, orderID),
	).Any()); err != nil {
		return err
	}

	// Give the customer 10 seconds to pay (canceled on payment).
	return ctx.Schedule("payment-due", time.Now().Add(10*time.Second))
}

func (w *OrderWorkflow) onPayment(ctx workflow.Ctx[PaymentReceivedData]) error {
	log.Printf("[workflow] payment received; completing workflow")

	if err := ctx.Unschedule("payment-due"); err != nil {
		return err
	}

	return ctx.Complete()
}

func (w *OrderWorkflow) onPaymentDue(ctx workflow.Ctx[workflow.TimeoutFiredData]) error {
	return ctx.Fail(errors.New("payment not received in time"))
}

func main() {
	log.SetFlags(0)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Command payloads must be registered so the command bus and the
	// workflow runtime can encode and decode them. The built-in workflow
	// events are registered automatically by workflow.NewService.
	reg := codec.New()
	codec.Register[OrderPlacedData](reg, OrderPlaced)
	codec.Register[PaymentReceivedData](reg, PaymentReceived)
	codec.Register[ReserveStockPayload](reg, ReserveStock)

	// In-memory backends. Any event.Bus, event.Store, and command.Bus work.
	ebus := eventbus.New()
	store := eventstore.New()

	cmdBus := cmdbus.New[int](reg, ebus)
	cmdBusErrs, err := cmdBus.Run(ctx)
	must(err)
	go drain("command bus", cmdBusErrs)

	// A regular command handler, standing in for the stock service.
	reserved := make(chan struct{}, 1)
	handlerErrs := command.MustHandle(ctx, cmdBus, ReserveStock, func(cmd command.Ctx[ReserveStockPayload]) error {
		log.Printf("[stock   ] reserving %d item(s) for order %s", cmd.Payload().ItemCount, cmd.Payload().OrderID)
		reserved <- struct{}{}
		return nil
	})
	go drain("command handler", handlerErrs)

	svc := workflow.NewService(workflow.Config{
		Commands:   reg,
		EventStore: store,
		EventBus:   ebus,
		CommandBus: cmdBus,
	}, OrderProcess)

	svcErrs, err := svc.Run(ctx)
	must(err)
	go drain("workflow service", svcErrs)

	// Publish the domain events that drive the workflow.
	orderID := uuid.New()

	log.Printf("[shop    ] placing order %s", orderID)
	must(ebus.Publish(ctx, event.New(
		OrderPlaced,
		OrderPlacedData{ItemCount: 3},
		event.Aggregate(orderID, OrderAggregate, 1),
	).Any()))

	repo := repository.New(store)
	await("workflow to start", func() bool {
		return status(repo, orderID) == workflow.StatusRunning
	})

	select {
	case <-reserved:
	case <-time.After(5 * time.Second):
		log.Fatal("timed out waiting for the stock reservation")
	}

	log.Printf("[shop    ] receiving payment for order %s", orderID)
	must(ebus.Publish(ctx, event.New(
		PaymentReceived,
		PaymentReceivedData{},
		event.Aggregate(orderID, OrderAggregate, 2),
	).Any()))

	await("workflow to complete", func() bool {
		return status(repo, orderID) == workflow.StatusCompleted
	})

	log.Printf("final status: %s", status(repo, orderID))
}

func status(repo *repository.Repository, id uuid.UUID) workflow.Status {
	w := NewOrderWorkflow(id)
	must(repo.Fetch(context.Background(), w))
	return w.Status()
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
