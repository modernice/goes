# 13. The Payment Workflow

Our shop has a gap: placing an order reserves stock, but nothing ensures the customer actually pays. An unpaid order keeps its stock forever — and even a cancelled order never returns it. Let's fix both with a durable [workflow](/guide/workflows).

## The Process

What we want:

- When an order is **placed**, a payment deadline starts ticking.
- If the order is **paid** in time, the process completes.
- If the deadline expires, the workflow **cancels the order** and **returns the stock** to the products.
- If the order is **cancelled** for any other reason, the stock comes back too.

A command handler can't do this — it would have to *wait* for a future `OrderPaid` event, enforce a deadline that survives restarts, and undo partial progress when things go wrong. That's exactly the job of a workflow: an event-sourced process manager that reacts to domain events, records durable command and timeout effects, and recovers from crashes.

## The Workflow

Create `payment_workflow.go`. Like our aggregates, the workflow gets its own file with everything colocated — and like an aggregate, it embeds a base type and persists its state through events:

```go
package shop

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/command"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/workflow"
)

const PaymentWorkflowAggregate = "shop.payment_workflow"

// PaymentDeadline is how long a customer has to pay an order. It's a variable
// so tests (and demos) can shorten it.
var PaymentDeadline = 30 * time.Minute

// Internal workflow event for rebuilding state.
const ItemsCaptured = "shop.payment_workflow.items_captured"

type ItemsCapturedData struct {
	Items []LineItem
}

type ItemsCapturedEvent = event.Of[ItemsCapturedData]

type PaymentWorkflow struct {
	*workflow.Base

	Items []LineItem
}

func NewPaymentWorkflow(id uuid.UUID) *PaymentWorkflow {
	w := &PaymentWorkflow{
		Base: workflow.New(PaymentWorkflowAggregate, id),
	}

	event.ApplyWith(w, w.itemsCaptured, ItemsCaptured)

	return w
}

func (w *PaymentWorkflow) captureItems(items []LineItem) {
	aggregate.Next(w, ItemsCaptured, ItemsCapturedData{Items: items})
}

func (w *PaymentWorkflow) itemsCaptured(evt ItemsCapturedEvent) {
	w.Items = evt.Data().Items
}

func RegisterPaymentWorkflowEvents(r codec.Registerer) {
	codec.Register[ItemsCapturedData](r, ItemsCaptured)
}
```

Why does the workflow capture the line items? Because it needs them *later*: when the order is cancelled, the workflow restocks each product — but the `OrderCancelled` event only carries the cancellation reason. So the workflow saves the items from `OrderPlaced` into its own state, using the same internal-event pattern our aggregates use: `aggregate.Next` records the event, the applier updates the in-memory state, and the state survives restarts because it's rebuilt from the event stream.

## The Definition

The definition wires domain events to handlers. Add it to `payment_workflow.go`:

```go
var PaymentProcess = workflow.Define(
	NewPaymentWorkflow,
	workflow.Starts(workflow.ByAggregateID, (*PaymentWorkflow).onOrderPlaced, OrderPlaced),
	workflow.Reacts(workflow.ByAggregateID, (*PaymentWorkflow).onOrderPaid, OrderPaid),
	workflow.Reacts(workflow.ByAggregateID, (*PaymentWorkflow).onOrderCancelled, OrderCancelled),
	workflow.Compensates(workflow.ByAggregateID, (*PaymentWorkflow).onCancelConfirmed, OrderCancelled),
	workflow.OnTimeout("payment", (*PaymentWorkflow).onPaymentDue),
	workflow.OnCompensationTimeout("cancel-order", (*PaymentWorkflow).onCancelTimeout),
)
```

Two things to notice:

**The correlator.** Every trigger event here comes from the Order aggregate, so `workflow.ByAggregateID` keys the workflow by the order's ID — one payment workflow per order, and the workflow's ID *is* the order's ID.

**`OrderCancelled` appears twice.** Once as `Reacts` and once as `Compensates`. The runtime picks the handler by phase: while the workflow is *running*, a cancellation is news from the outside (the customer cancelled), and the `Reacts` handler runs. While the workflow is *compensating*, the cancellation is the confirmation of the cancel command the workflow itself dispatched, and the `Compensates` handler runs.

## The Handlers

The handlers orchestrate the process. Add them to `payment_workflow.go`:

```go
func (w *PaymentWorkflow) onOrderPlaced(ctx workflow.Ctx[OrderPlacedData]) error {
	w.captureItems(ctx.Event().Data().Items)

	return ctx.Schedule("payment", time.Now().Add(PaymentDeadline))
}

func (w *PaymentWorkflow) onOrderPaid(ctx workflow.Ctx[int]) error {
	// Complete() cancels all active timeouts, including "payment".
	return ctx.Complete()
}

func (w *PaymentWorkflow) onPaymentDue(ctx workflow.Ctx[workflow.TimeoutFiredData]) error {
	if err := ctx.Compensate(errors.New("payment deadline missed")); err != nil {
		return err
	}

	if err := ctx.Dispatch("cancel-order", command.New(
		CancelOrderCmd,
		"payment deadline missed",
		command.Aggregate(OrderAggregate, w.AggregateID()),
	).Any()); err != nil {
		return err
	}

	// Safety net: if the cancellation isn't confirmed within a minute,
	// flag the workflow for manual intervention.
	return ctx.Schedule("cancel-order", time.Now().Add(time.Minute))
}

func (w *PaymentWorkflow) onCancelConfirmed(ctx workflow.Ctx[string]) error {
	if err := w.restock(ctx); err != nil {
		return err
	}

	return ctx.Compensated()
}

func (w *PaymentWorkflow) onOrderCancelled(ctx workflow.Ctx[string]) error {
	// The customer cancelled the order themselves — the cancellation reason
	// becomes the workflow's Reason().
	if err := ctx.Compensate(errors.New(ctx.Event().Data())); err != nil {
		return err
	}

	if err := w.restock(ctx); err != nil {
		return err
	}

	return ctx.Compensated()
}

func (w *PaymentWorkflow) onCancelTimeout(ctx workflow.Ctx[workflow.TimeoutFiredData]) error {
	return ctx.CompensationFailed(errors.New("order cancellation was not confirmed"))
}

func (w *PaymentWorkflow) restock(ctx workflow.Ctx[string]) error {
	for _, item := range w.Items {
		if err := ctx.Dispatch("restock-"+item.ProductID.String(), command.New(
			AdjustStockCmd,
			AdjustStockPayload{Quantity: item.Quantity, Reason: "order cancelled"},
			command.Aggregate(ProductAggregate, item.ProductID),
		).Any()); err != nil {
			return err
		}
	}

	return nil
}
```

Walking through the paths:

1. **Happy path** — `onOrderPlaced` saves the line items and schedules the `"payment"` deadline. `onOrderPaid` completes the workflow, which automatically cancels the deadline.
2. **Deadline missed** — `onPaymentDue` fires. It enters compensation, dispatches `CancelOrderCmd` against the order, and schedules a compensation-phase safety net. When the order's `OrderCancelled` event comes back, the workflow is compensating, so `onCancelConfirmed` runs: it dispatches one `AdjustStockCmd` per line item and marks the compensation complete.
3. **Manual cancellation** — the customer cancels an unpaid order. The workflow is still running, so `onOrderCancelled` runs: it enters *and* finishes compensation in one handler — the order is already cancelled, only the restocking is left to do.
4. **Cancellation not confirmed** — if `CancelOrderCmd` doesn't result in an `OrderCancelled` event (say, a payment slipped in right before the deadline, so the order is no longer open), the `"cancel-order"` safety net fires and `onCancelTimeout` marks the compensation as failed — a signal for a human to look at the order.

Note what the handlers *don't* do: they never talk to a repository, never publish events, never sleep. `ctx.Dispatch` and `ctx.Schedule` only **record** effects as events on the workflow — a background runtime executes them after the workflow is saved, and re-executes them after a crash. That's what makes the process durable.

::: tip Advanced Reading
The [Workflows guide](/guide/workflows) covers the full runtime semantics — exactly-once trigger handling, at-least-once command dispatch, running multiple service instances — and the [cookbook](/reference/cookbook#workflow-status-dashboard) shows how to build a dashboard that finds workflows stuck in compensation.
:::

## Wiring It Up

Update `cmd/main.go` to register the new events and run the workflow service. The built-in `goes.workflow.*` events must be registered in the event registry so the MongoDB store and NATS bus can encode them (the command registry gets them automatically):

```go
import (
	// ... existing imports ...
	"github.com/modernice/goes/workflow" // [!code ++]
)

	// ... existing event registrations ...
	shop.RegisterPaymentWorkflowEvents(eventReg) // [!code ++]
	workflow.RegisterEvents(eventReg)            // [!code ++]
```

Then start the service alongside the command handlers and projections:

```go
	// ... existing projections ...

	payments := workflow.NewService(workflow.Config{ // [!code ++]
		EventStore: store, // [!code ++]
		EventBus:   bus,   // [!code ++]
		CommandBus: cbus,  // [!code ++]
		Commands:   cmdReg, // [!code ++]
		// Recover triggers that were missed while the process was down. // [!code ++]
		// NATS Core doesn't redeliver on its own, so the service replays // [!code ++]
		// recent events from the event store on startup. // [!code ++]
		TriggerReplayWindow: 24 * time.Hour, // [!code ++]
	}, shop.PaymentProcess) // [!code ++]

	paymentErrs, err := payments.Run(ctx) // [!code ++]
	if err != nil {                       // [!code ++]
		log.Fatalf("Failed to run payment workflow: %v", err) // [!code ++]
	}                                     // [!code ++]
	go logErrors(paymentErrs)             // [!code ++]
```

The service subscribes to the trigger events (`OrderPlaced`, `OrderPaid`, `OrderCancelled`), persists workflow instances through the event store, and runs the effect runtime that dispatches the recorded commands and fires due timeouts. Like everything else in our app, its error channel is drained with `logErrors`.

> [!NOTE]
> We run a single instance of the shop, so reusing the one NATS bus for events and commands is fine. When you scale a workflow service to multiple instances, give the trigger bus a load-balanced (queue group) configuration and keep the command bus on a separate, non-load-balanced transport — see [Scaling](/guide/workflows#scaling) in the guide.

## Watch It Work

To see the workflow in action without waiting half an hour, temporarily shorten the deadline and place an order without paying:

```go
	shop.PaymentDeadline = time.Minute

	// ... after the services are running ...

	productID := uuid.New()
	p := shop.NewProduct(productID)
	p.Create("Wireless Mouse", 10)
	products.Save(ctx, p)

	orderID := uuid.New()
	cbus.Dispatch(ctx, command.New(shop.PlaceOrderCmd, shop.PlaceOrderPayload{
		CustomerID: uuid.New(),
		Items: []shop.LineItem{
			{ProductID: productID, Name: "Wireless Mouse", Price: 2999, Quantity: 2},
		},
	}, command.Aggregate(shop.OrderAggregate, orderID)).Any())
```

Placing the order drops the product's stock from 10 to 8. One minute later, the workflow cancels the order and the stock returns to 10 — restart the process in between if you like, the deadline survives.

## Testing the Workflow

The whole loop can be tested with in-memory backends, in the same style as [chapter 12](./12-testing). Create `payment_workflow_test.go`:

```go
func TestPaymentWorkflow_DeadlineMissed(t *testing.T) {
	restore := shop.PaymentDeadline
	shop.PaymentDeadline = 200 * time.Millisecond
	defer func() { shop.PaymentDeadline = restore }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus := eventbus.New()
	store := eventstore.WithBus(eventstore.New(), bus)

	repo := repository.New(store)
	products := repository.Typed(repo, shop.NewProduct)
	orders := repository.Typed(repo, shop.NewOrder)

	cmdReg := codec.New()
	shop.RegisterProductCommands(cmdReg)
	shop.RegisterOrderCommands(cmdReg)

	cbus := cmdbus.New[int](cmdReg, bus)
	go streams.Drain(shop.HandleProductCommands(ctx, cbus, products))
	go streams.Drain(shop.HandleOrderCommands(ctx, cbus, orders, products))

	svc := workflow.NewService(workflow.Config{
		EventStore: store,
		EventBus:   bus,
		CommandBus: cbus,
		Commands:   cmdReg,
	}, shop.PaymentProcess)

	wfErrs, err := svc.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	go streams.Drain(wfErrs)

	// Seed a product with stock.
	productID := uuid.New()
	p := shop.NewProduct(productID)
	if err := p.Create("Wireless Mouse", 50); err != nil {
		t.Fatal(err)
	}
	if err := products.Save(ctx, p); err != nil {
		t.Fatal(err)
	}

	// Place an order — and never pay it.
	orderID := uuid.New()
	cmd := command.New(shop.PlaceOrderCmd, shop.PlaceOrderPayload{
		CustomerID: uuid.New(),
		Items: []shop.LineItem{
			{ProductID: productID, Name: "Wireless Mouse", Price: 2999, Quantity: 2},
		},
	}, command.Aggregate(shop.OrderAggregate, orderID))
	if err := cbus.Dispatch(ctx, cmd.Any()); err != nil {
		t.Fatal(err)
	}

	// Wait for the deadline to expire and the compensation to run.
	time.Sleep(2 * time.Second)

	order, err := orders.Fetch(ctx, orderID)
	if err != nil {
		t.Fatal(err)
	}
	if !order.Cancelled() {
		t.Fatalf("order should be cancelled, is %v", order.Status)
	}

	product, err := products.Fetch(ctx, productID)
	if err != nil {
		t.Fatal(err)
	}
	if product.Stock != 50 {
		t.Errorf("stock should be restored to 50, is %d", product.Stock)
	}

	workflows := repository.Typed(repo, shop.NewPaymentWorkflow)
	wf, err := workflows.Fetch(ctx, orderID)
	if err != nil {
		t.Fatal(err)
	}
	if wf.Status() != workflow.StatusCompensated {
		t.Errorf("workflow should be compensated, is %q (reason: %q)", wf.Status(), wf.Reason())
	}
}
```

The test drives the real flow end to end: the `PlaceOrderCmd` handler decrements the stock, the workflow's deadline expires, the cancel command flips the order to cancelled, and the compensation restocks the product. The final assertions read the workflow instance itself — it's just an aggregate, fetched through a typed repository like any other.

## Patterns to Notice

1. **Handlers record, the runtime executes.** `ctx.Dispatch` and `ctx.Schedule` persist effects as events on the workflow. Nothing happens inline — which is why a crash between "decided to cancel" and "cancelled" loses nothing.

2. **Outcomes come back as events.** The workflow never checks whether `CancelOrderCmd` "succeeded" — it reacts to the `OrderCancelled` event, the same way it would if anyone else had cancelled the order. Commands go out, events come back.

3. **One event, two phases.** Registering `OrderCancelled` as both `Reacts` and `Compensates` gives the same fact different meanings depending on where the process stands.

4. **Compensation is explicit.** Entering it (`ctx.Compensate`), doing the undo work (restocking), and finishing it (`ctx.Compensated` / `ctx.CompensationFailed`) are deliberate steps in your handlers — the runtime doesn't guess how to undo an order.

5. **Deadlines are just state.** The `"payment"` timeout lives in the workflow's event stream, so it survives restarts and fires on whichever service instance is alive when it's due.

## What You've Built

Congratulations! You've built a complete event-sourced e-commerce application with:

- **4 aggregates** — Product, Pricing, Order, Customer
- **Type-safe events and commands** — with codec registration
- **Typed repositories** — persist and fetch aggregates
- **A product catalog projection** — reactive read model
- **A payment workflow** — durable deadlines with automatic compensation
- **Production backends** — MongoDB event store, NATS event bus
- **Tests** — for aggregates, projections, command handlers, and workflows

## What's Next?

- **[Guide](/guide/aggregates)** — deep-dive into each framework component
- **[Workflows](/guide/workflows)** — the full workflow runtime: delivery guarantees, snapshots, scaling
- **[Backends](/backends/)** — detailed backend configuration and options
- **[Reference](/reference/architecture)** — architecture overview and best practices
