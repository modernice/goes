// Command compensation demonstrates the compensation phase of a workflow:
//
//   - Ctx.Compensate switches a workflow to StatusCompensating after a
//     business failure; from then on only workflow.Compensates and
//     workflow.OnCompensationTimeout handlers run
//   - Ctx.Compensated finishes a successful compensation
//     (StatusCompensated)
//   - Ctx.CompensationFailed gives up on a compensation (StatusFailed),
//     here driven by a compensation timeout
//
// Two orders run through the same process: order A compensates successfully
// (stock released), while order B's stock release gets lost, so its
// compensation deadline fires and the workflow fails — the case a human
// operator needs to look at.
package main

import (
	"context"
	"errors"
	"log"
	"sync"
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

	OrderPlaced   = "shop.order.placed"
	StockReserved = "shop.order.stock_reserved"
	PaymentFailed = "shop.order.payment_failed"
	StockReleased = "shop.order.stock_released"

	ReserveStock  = "shop.reserve_stock"
	ChargePayment = "shop.charge_payment"
	ReleaseStock  = "shop.release_stock"

	OrderWorkflowName = "shop.order_workflow"

	releaseDeadline = 600 * time.Millisecond
)

type OrderPlacedData struct{}
type StockReservedData struct{}
type PaymentFailedData struct{ Reason string }
type StockReleasedData struct{}

type StockPayload struct{ OrderID uuid.UUID }
type ChargePayload struct{ OrderID uuid.UUID }

// OrderWorkflow reserves stock, charges the payment, and releases the stock
// again if the payment fails.
type OrderWorkflow struct {
	*workflow.Base
}

func NewOrderWorkflow(id uuid.UUID) *OrderWorkflow {
	return &OrderWorkflow{Base: workflow.New(OrderWorkflowName, id)}
}

var OrderProcess = workflow.Define(
	NewOrderWorkflow,
	workflow.Starts(workflow.ByAggregateID, (*OrderWorkflow).onPlaced, OrderPlaced),
	workflow.Reacts(workflow.ByAggregateID, (*OrderWorkflow).onReserved, StockReserved),
	workflow.Reacts(workflow.ByAggregateID, (*OrderWorkflow).onPaymentFailed, PaymentFailed),

	// StockReleased only advances the workflow while it is compensating; a
	// workflow.Reacts registration for the same event would handle it during
	// the forward phase instead.
	workflow.Compensates(workflow.ByAggregateID, (*OrderWorkflow).onStockReleased, StockReleased),
	workflow.OnCompensationTimeout("release-due", (*OrderWorkflow).onReleaseOverdue),
)

func (w *OrderWorkflow) onPlaced(ctx workflow.Ctx[OrderPlacedData]) error {
	orderID, _, _ := ctx.Event().Aggregate()
	log.Printf("[workflow] order %s: reserving stock", short(orderID))

	return ctx.Dispatch("reserve", command.New(
		ReserveStock, StockPayload{OrderID: orderID}, command.Aggregate(OrderAggregate, orderID),
	).Any())
}

func (w *OrderWorkflow) onReserved(ctx workflow.Ctx[StockReservedData]) error {
	orderID, _, _ := ctx.Event().Aggregate()
	log.Printf("[workflow] order %s: stock reserved, charging payment", short(orderID))

	return ctx.Dispatch("charge", command.New(
		ChargePayment, ChargePayload{OrderID: orderID}, command.Aggregate(OrderAggregate, orderID),
	).Any())
}

func (w *OrderWorkflow) onPaymentFailed(ctx workflow.Ctx[PaymentFailedData]) error {
	orderID, _, _ := ctx.Event().Aggregate()
	log.Printf("[workflow] order %s: payment failed, compensating", short(orderID))

	// Enter compensation and undo the stock reservation. If the release
	// does not happen in time, the compensation deadline fires.
	if err := ctx.Compensate(errors.New(ctx.Event().Data().Reason)); err != nil {
		return err
	}

	if err := ctx.Dispatch("release", command.New(
		ReleaseStock, StockPayload{OrderID: orderID}, command.Aggregate(OrderAggregate, orderID),
	).Any()); err != nil {
		return err
	}

	return ctx.Schedule("release-due", time.Now().Add(releaseDeadline))
}

func (w *OrderWorkflow) onStockReleased(ctx workflow.Ctx[StockReleasedData]) error {
	orderID, _, _ := ctx.Event().Aggregate()
	log.Printf("[workflow] order %s: stock released, compensation complete", short(orderID))

	return ctx.Compensated()
}

func (w *OrderWorkflow) onReleaseOverdue(ctx workflow.Ctx[workflow.TimeoutFiredData]) error {
	log.Printf("[workflow] order %s: stock release overdue, giving up", short(ctx.Event().Data().WorkflowID))

	return ctx.CompensationFailed(errors.New("stock release timed out"))
}

func main() {
	log.SetFlags(0)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reg := codec.New()
	codec.Register[OrderPlacedData](reg, OrderPlaced)
	codec.Register[StockReservedData](reg, StockReserved)
	codec.Register[PaymentFailedData](reg, PaymentFailed)
	codec.Register[StockReleasedData](reg, StockReleased)
	codec.Register[StockPayload](reg, ReserveStock)
	codec.Register[ChargePayload](reg, ChargePayment)
	codec.Register[StockPayload](reg, ReleaseStock)

	ebus := eventbus.New()
	store := eventstore.New()

	cmdBus := cmdbus.New[int](reg, ebus)
	cmdBusErrs, err := cmdBus.Run(ctx)
	must(err)
	go drain("command bus", cmdBusErrs)

	// lostReleases simulates an unreliable stock service: release commands
	// for these orders are acknowledged but never performed.
	var mu sync.Mutex
	lostReleases := map[uuid.UUID]bool{}

	// The stock and payment services react to the workflow's commands by
	// publishing domain events, which drive the workflow forward.
	go drain("reserve handler", command.MustHandle(ctx, cmdBus, ReserveStock, func(cmd command.Ctx[StockPayload]) error {
		log.Printf("[stock   ] order %s: reserved", short(cmd.Payload().OrderID))
		return ebus.Publish(ctx, event.New(
			StockReserved, StockReservedData{}, event.Aggregate(cmd.Payload().OrderID, OrderAggregate, 2),
		).Any())
	}))

	go drain("charge handler", command.MustHandle(ctx, cmdBus, ChargePayment, func(cmd command.Ctx[ChargePayload]) error {
		log.Printf("[payment ] order %s: card declined", short(cmd.Payload().OrderID))
		return ebus.Publish(ctx, event.New(
			PaymentFailed, PaymentFailedData{Reason: "card declined"}, event.Aggregate(cmd.Payload().OrderID, OrderAggregate, 3),
		).Any())
	}))

	go drain("release handler", command.MustHandle(ctx, cmdBus, ReleaseStock, func(cmd command.Ctx[StockPayload]) error {
		mu.Lock()
		lost := lostReleases[cmd.Payload().OrderID]
		mu.Unlock()

		if lost {
			log.Printf("[stock   ] order %s: release request lost 📉", short(cmd.Payload().OrderID))
			return nil
		}

		log.Printf("[stock   ] order %s: released", short(cmd.Payload().OrderID))
		return ebus.Publish(ctx, event.New(
			StockReleased, StockReleasedData{}, event.Aggregate(cmd.Payload().OrderID, OrderAggregate, 4),
		).Any())
	}))

	svc := workflow.NewService(workflow.Config{
		Commands:         reg,
		EventStore:       store,
		EventBus:         ebus,
		CommandBus:       cmdBus,
		DispatchInterval: 25 * time.Millisecond,
		TimerResolution:  10 * time.Millisecond,
	}, OrderProcess)

	svcErrs, err := svc.Run(ctx)
	must(err)
	go drain("workflow service", svcErrs)

	repo := repository.New(store)

	orderA := uuid.New()
	orderB := uuid.New()

	mu.Lock()
	lostReleases[orderB] = true
	mu.Unlock()

	log.Printf("--- order A (%s): compensation succeeds ---", short(orderA))
	place(ctx, ebus, orderA)
	await("order A to be compensated", func() bool {
		return fetch(repo, orderA).Status() == workflow.StatusCompensated
	})

	log.Printf("--- order B (%s): compensation fails ---", short(orderB))
	place(ctx, ebus, orderB)
	await("order B to fail", func() bool {
		return fetch(repo, orderB).Status() == workflow.StatusFailed
	})

	a, b := fetch(repo, orderA), fetch(repo, orderB)
	log.Printf("order A: status=%s reason=%q", a.Status(), a.Reason())
	log.Printf("order B: status=%s reason=%q", b.Status(), b.Reason())
}

func place(ctx context.Context, bus event.Bus, orderID uuid.UUID) {
	must(bus.Publish(ctx, event.New(
		OrderPlaced, OrderPlacedData{}, event.Aggregate(orderID, OrderAggregate, 1),
	).Any()))
}

func fetch(repo *repository.Repository, id uuid.UUID) *OrderWorkflow {
	w := NewOrderWorkflow(id)
	must(repo.Fetch(context.Background(), w))
	return w
}

func short(id uuid.UUID) string {
	return id.String()[:8]
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
