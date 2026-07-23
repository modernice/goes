# Workflows

Workflows coordinate multi-step business processes that unfold over time — also known as *sagas* or *process managers*. When a process spans multiple events, commands, timeouts, and compensation steps — potentially across aggregates or services — a workflow orchestrates the sequence durably and recovers from failures.

> See also: [Commands](/guide/commands) for single-step dispatch, [Aggregates](/guide/aggregates) for consistency boundaries, [Event Handlers](/guide/event-handlers) for lightweight observers.

::: info Coming from the `saga` package?
The `workflow` package supersedes the deprecated in-process `saga` package. See [Migrating from the saga Package](#migrating-from-the-saga-package).
:::

## When to Use a Workflow

A workflow is a good fit when the process:

- waits for future events before deciding the next step
- sets deadlines (timeouts) that trigger fallback behavior
- crosses service or process boundaries
- needs explicit compensation to undo partial progress

You don't need a workflow when the entire rule fits inside a single [aggregate](/guide/aggregates) consistency boundary, or when a single [command handler](/guide/commands) can execute the whole operation immediately. A workflow coordinates *between* aggregates and services — the aggregates still own their own state and invariants, while the workflow drives the overall process forward.

## Mental Model

If you've worked with [aggregates](/guide/aggregates), workflows will feel familiar — under the hood, a workflow instance *is* an event-sourced aggregate:

- It embeds `*workflow.Base`.
- External domain events start or advance it.
- Handlers never perform side effects directly — they record durable effects as events on the workflow instead.
- A background effect runtime later dispatches pending commands and fires due timeouts.
- State, pending commands, and active timeouts survive restarts because they are rebuilt from the event store — any service instance can pick up where a crashed one left off.

Two delivery guarantees shape the model:

- **Trigger events are handled exactly once** per event ID — handled trigger IDs are persisted atomically with the handler's changes, so redelivered or replayed events are ignored.
- **Effects are at-least-once.** Commands and timeout-fired events may be retried after a crash, so downstream handlers and aggregates should be idempotent.

## Defining a Workflow

A workflow definition has three parts: a struct that holds the workflow's state, a constructor, and a set of registrations that tell the runtime which events to listen for and how to handle them.

### `workflow.Base` and `workflow.New`

Embed `*workflow.Base` in your workflow struct. The constructor calls `workflow.New` with the aggregate name and ID:

```go
type OrderWorkflow struct {
	*workflow.Base

	WarehouseID uuid.UUID
	Items       []LineItem
}

func NewOrderWorkflow(id uuid.UUID) *OrderWorkflow {
	w := &OrderWorkflow{
		Base: workflow.New("shop.order_workflow", id),
	}

	// Register appliers for internal workflow events (shown in the full example below).
	event.ApplyWith(w, w.orderCaptured, OrderCaptured)
	event.ApplyWith(w, w.stockReleaseRequested, StockReleaseRequested)

	return w
}
```

`Base` provides the event-sourced lifecycle state, trigger deduplication, pending command effects, and active timeout tracking. If the workflow needs its own state beyond what `Base` tracks, persist it through internal events and register appliers with `event.ApplyWith` — the same way you would in an [aggregate](/guide/aggregates). The [full example](#full-example) below shows this in detail.

### `workflow.Define`

`workflow.Define` ties a typed constructor to one or more registrations. This is the unit you pass to `workflow.NewService(...)`:

```go
var OrderProcess = workflow.Define(
	NewOrderWorkflow,
	workflow.Starts(...),
	workflow.Reacts(...),
	workflow.Compensates(...),
	workflow.OnTimeout(...),
	workflow.OnCompensationTimeout(...),
)
```

### Registration Functions

Each `Starts`, `Reacts`, and `Compensates` registration takes a **correlator**, a **handler**, and one or more **event names**. Timeout registrations take a **timeout key** and a **handler**:

| Function | Phase | Purpose |
| --- | --- | --- |
| `workflow.Starts(correlator, handler, events...)` | Forward | Creates a new workflow instance if one doesn't exist for the correlated ID; behaves like `Reacts` if it does |
| `workflow.Reacts(correlator, handler, events...)` | Forward | Advances the workflow during normal progression |
| `workflow.Compensates(correlator, handler, events...)` | Compensation | Runs only after the workflow entered compensation via `ctx.Compensate(...)` |
| `workflow.OnTimeout(key, handler)` | Forward | Fires when a forward-phase timeout expires |
| `workflow.OnCompensationTimeout(key, handler)` | Compensation | Fires when a compensation-phase timeout expires |

Events that correlate to an unknown workflow are ignored by `Reacts` and `Compensates`, unless the service runs in [strict mode](#workflow-config). Once a workflow reaches a terminal status, further trigger events are ignored entirely.

## Correlators

When a domain event arrives, the workflow runtime needs to figure out *which workflow instance* should handle it. That's the correlator's job — it inspects the event and returns the UUID of the workflow instance it belongs to:

```go
type Correlator[Data any] func(event.Of[Data]) (uuid.UUID, bool)
```

Return `false` as the second value to skip the event.

### `workflow.ByAggregateID`

The most common case is keying the workflow by the aggregate that emitted the event — one workflow instance per aggregate. `ByAggregateID` is passed uninstantiated; its type argument is inferred from the handler:

```go
workflow.Starts(
	workflow.ByAggregateID,
	(*OrderWorkflow).onOrderPlaced,
	OrderPlaced,
)
```

### `workflow.ByKey`

`ByKey` derives the workflow ID deterministically (UUIDv5) from a business key in the event payload — one workflow instance per key. All events that yield the same key correlate to the same workflow, regardless of which aggregate emitted them:

```go
var customers = uuid.MustParse("d3f0a1de-52a7-40e8-8a3a-79dbd8f2071e")

workflow.Starts(
	workflow.ByKey(customers, func(d OrderPlacedData) string { return d.CustomerEmail }),
	(*LoyaltyWorkflow).onOrder,
	OrderPlaced,
)
```

The first argument is a UUID namespace of your choosing — it isolates the derived IDs of this workflow type from other ID derivations in your system. Events for which the key function returns an empty string are ignored.

### Custom correlators

For workflows where the ID comes from a field in the event data, write the correlator yourself:

```go
workflow.Reacts(
	func(evt event.Of[PaymentReceivedData]) (uuid.UUID, bool) {
		return evt.Data().OrderID, true
	},
	(*OrderWorkflow).onPaymentReceived,
	PaymentReceived,
)
```

In practice, cross-aggregate events (like `PaymentReceived` from the payment aggregate) should include the correlation ID (e.g. `OrderID`) in their event data — this is the simplest and most common approach. When the producing aggregate genuinely doesn't know about the workflow's domain (e.g. a generic inventory service), the correlator can use an injected service or [lookup](/guide/lookups) to resolve the workflow ID through more indirection.

## Handler Context

Every workflow handler receives a `workflow.Ctx[D]` — the main interface for interacting with the workflow runtime. It embeds `context.Context` and exposes the triggering event via `ctx.Event()`, along with methods to record effects and transition the workflow's lifecycle.

### Recording effects

These methods record durable workflow events — they don't execute anything immediately. The effects are persisted together with the workflow's other changes, and the background effect runtime dispatches pending commands and fires due timeouts later.

#### `Dispatch(key, cmd)`

Records an outgoing [command](/guide/commands) for background dispatch. The `key` identifies the command within the current trigger — the runtime uses it together with the workflow ID and trigger event ID to compute a deterministic effect ID, so calling `Dispatch` with the same key and command for the same trigger is an idempotent no-op. Recording the same key with a *different* command returns `workflow.ErrEffectConflict`.

The command must be passed as a `command.Command` (i.e. `command.Of[any]`). Since `command.New` returns a typed `command.Cmd[P]`, call `.Any()` to erase the type parameter:

```go
ctx.Dispatch("reserve-stock", command.New(
	ReserveStockCmd,
	ReserveStockPayload{OrderID: orderID},
	command.Aggregate(OrderAggregate, orderID),
).Any())
```

The workflow serializes the command payload immediately using the codec from `workflow.Config.Commands` (shown in [Running the Service](#running-the-service) below), so the payload type must be registered in that codec. The actual dispatch happens later in the effect runtime, with automatic retries on failure (controlled by `workflow.Config.DispatchInterval`).

::: info Command outcomes come back as events
Dispatch is fire-and-forget: the effect runtime retries *bus-level* dispatch failures, but the command handler's return value never flows back into the workflow. Model outcomes as domain events instead — the target aggregate emits success or failure events (`PaymentReceived`, `PaymentDeclined`), and the workflow `Reacts` to those. This is why the [full example](#full-example) below advances on domain events rather than command results.
:::

#### `Schedule(key, at)`

Records a named deadline. When the clock reaches `at`, the effect runtime fires the timeout and the corresponding `OnTimeout` (or `OnCompensationTimeout`) handler runs.

```go
ctx.Schedule("payment", time.Now().Add(15*time.Minute))
```

If a timeout with the same key already exists, `Schedule` cancels the old one and replaces it with the new deadline. If the key and time are identical to the existing timeout, it's a no-op. The polling frequency of the timeout timer is controlled by `workflow.Config.TimerResolution`.

#### `Unschedule(key)`

Cancels the active timeout with the given key. If no timeout with that key exists, it's a no-op.

```go
ctx.Unschedule("payment")
```

#### Recurring timeouts

A fired timeout is consumed — its key becomes free again. Scheduling the same key from inside the `OnTimeout` handler therefore arms a fresh deadline, which turns a one-shot timeout into a reminder or escalation loop. Persist the round counter through an internal workflow event (the same [state pattern](#workflow-struct-constructor-and-state-methods) as in the full example) so the loop survives restarts:

```go
workflow.OnTimeout("payment", (*OrderWorkflow).onPaymentDue)

func (w *OrderWorkflow) onPaymentDue(ctx workflow.Ctx[workflow.TimeoutFiredData]) error {
	if w.RemindersSent >= 3 {
		return ctx.Fail(errors.New("payment overdue after 3 reminders"))
	}

	// Records an internal event whose applier increments w.RemindersSent.
	w.recordReminderSent()

	if err := ctx.Dispatch("payment-reminder", command.New(
		SendPaymentReminderCmd,
		SendPaymentReminderPayload{OrderID: w.AggregateID()},
		command.Aggregate(OrderAggregate, w.AggregateID()),
	).Any()); err != nil {
		return err
	}

	return ctx.Schedule("payment", time.Now().Add(24*time.Hour))
}
```

Each firing is a distinct trigger event, so effect keys reused across rounds (like `"payment-reminder"` here) produce fresh effects — the reminder command is dispatched once per round, not deduplicated across rounds. After the last round, transition instead of rescheduling: `Fail` the workflow, or `Compensate` and let the compensation phase clean up.

### Transitioning the workflow

These methods move the workflow through its lifecycle. Each one also cancels all active timeouts before transitioning.

| Method | From | To | Description |
| --- | --- | --- | --- |
| `Complete()` | Running | Completed | The process finished successfully. |
| `Fail(err)` | Running | Failed | The process failed. No compensation. |
| `Compensate(err)` | Running | Compensating | Something went wrong — begin undoing side effects. |
| `Compensated()` | Compensating | Compensated | Compensation succeeded. The workflow is over. |
| `CompensationFailed(err)` | Compensating | Failed | Compensation itself failed. Needs manual intervention. |

This yields five lifecycle statuses:

| Status | Terminal | Meaning |
| --- | --- | --- |
| `StatusRunning` | — | Started, not finished. |
| `StatusCompleted` | ✓ | Finished successfully. |
| `StatusCompensating` | — | Undoing previous work after a business failure. |
| `StatusCompensated` | ✓ | Compensation finished successfully. |
| `StatusFailed` | ✓ | Failed directly, or compensation failed. |

Calling a transition from the wrong status returns an error (`ErrInvalidTransition` or `ErrNotCompensating`). Repeating a transition the workflow already made — e.g. calling `Complete()` on a completed workflow — is a safe no-op.

Query the outcome through `Base`:

- `Status()` returns the current lifecycle status.
- `Reason()` returns why the workflow left the happy path — the reason of the most recent `Fail`, `Compensate`, or `CompensationFailed` transition.
- `Done()` reports whether the workflow reached a terminal status.

### Handler Errors vs. Business Failures

A trigger is processed atomically: only if the handler returns `nil` are the workflow's changes — including the record of the handled trigger — persisted. If the handler returns an error, *nothing* is saved. The error is reported on the error channel returned by `Run`, and the runtime does **not** retry the trigger by itself. Because the trigger was never recorded, the event remains unhandled from the workflow's perspective: it is re-processed if it arrives again — through bus-level redelivery (transport-dependent), or through [startup replay](#trigger-replay) when the service restarts with `TriggerReplayWindow` enabled.

This makes returning an error the right move only for *transient, infrastructure-level* problems — a failed external read, an encoding failure — where dropping the trigger and re-processing it later is what you want. A declined payment or an out-of-stock item is not an error; it's a business outcome the workflow must record durably:

| Situation | Do this | Effect |
| --- | --- | --- |
| Transient failure (dependency down, encoding error) | `return err` | Nothing persisted; error surfaces on the error channel; trigger re-processed on redelivery or replay |
| Business failure (payment declined, deadline missed) | `ctx.Fail(err)` or `ctx.Compensate(err)` | Failure becomes part of the workflow's durable history; trigger recorded as handled |

Watch the error channel in production — a handler that keeps failing shows up there, not in the workflow's state.

## Full Example

This example models an order process with forward progression and compensation:

- `OrderPlaced` starts the workflow, dispatches a stock reservation, and sets a 15-minute payment deadline
- `PaymentReceived` cancels the timeout and completes the workflow
- `PaymentDeclined` enters compensation and dispatches a stock release
- `StockReleased` finishes compensation
- Timeouts fail their respective phases

### Domain types

```go
const OrderWorkflowAggregate = "shop.order_workflow"
const OrderAggregate = "shop.order"

// Domain events.
const (
	OrderPlaced     = "shop.order.placed"
	PaymentReceived = "shop.payment.received"
	PaymentDeclined = "shop.payment.declined"
	StockReleased   = "shop.stock.released"
)

// Commands.
const (
	ReserveStockCmd = "shop.stock.reserve"
	ReleaseStockCmd = "shop.stock.release"
)

// Internal workflow events for rebuilding state.
const (
	OrderCaptured         = "shop.order_workflow.order_captured"
	StockReleaseRequested = "shop.order_workflow.stock_release_requested"
)
```

Each event and command name has a corresponding payload struct (e.g. `OrderPlacedData`, `ReserveStockPayload`). The handlers below access these through `ctx.Event().Data()`. The payload structs themselves are straightforward data carriers, so they're omitted here for brevity.

### Workflow struct, constructor, and state methods

```go
type OrderWorkflow struct {
	*workflow.Base

	WarehouseID         uuid.UUID
	Items               []LineItem
	StockReleasePending bool
	StockReleaseReason  string
}

func NewOrderWorkflow(id uuid.UUID) *OrderWorkflow {
	w := &OrderWorkflow{
		Base: workflow.New(OrderWorkflowAggregate, id),
	}

	event.ApplyWith(w, w.orderCaptured, OrderCaptured)
	event.ApplyWith(w, w.stockReleaseRequested, StockReleaseRequested)

	return w
}

// captureOrder records the order data so compensation handlers can reference it later.
func (w *OrderWorkflow) captureOrder(data OrderPlacedData) {
	aggregate.Next(w, OrderCaptured, OrderCapturedData{
		WarehouseID: data.WarehouseID,
		Items:       data.Items,
	})
}

func (w *OrderWorkflow) orderCaptured(evt event.Of[OrderCapturedData]) {
	w.WarehouseID = evt.Data().WarehouseID
	w.Items = evt.Data().Items
}

// requestStockRelease marks that a stock release is pending.
func (w *OrderWorkflow) requestStockRelease(reason string) {
	aggregate.Next(w, StockReleaseRequested, StockReleaseRequestedData{
		Reason: reason,
	})
}

func (w *OrderWorkflow) stockReleaseRequested(evt event.Of[StockReleaseRequestedData]) {
	w.StockReleasePending = true
	w.StockReleaseReason = evt.Data().Reason
}
```

Each business method (e.g. `captureOrder`) records an internal workflow event with `aggregate.Next`, and the corresponding applier (e.g. `orderCaptured`) updates the workflow's in-memory state when that event is replayed. This is the same pattern used in [aggregates](/guide/aggregates) — the internal events persist state durably so it survives restarts.

### Definition

This is where the event wiring comes together. The definition binds each domain event to a correlator and handler, and registers timeout handlers by key. Only `OrderPlaced` uses `workflow.ByAggregateID` because it originates from the order aggregate — its aggregate ID *is* the order ID. The other events come from the payment and stock aggregates, so they use custom correlators that extract the `OrderID` from the event data.

```go
var OrderProcess = workflow.Define(
	NewOrderWorkflow,
	workflow.Starts(
		workflow.ByAggregateID,
		(*OrderWorkflow).onOrderPlaced,
		OrderPlaced,
	),
	workflow.Reacts(
		func(evt event.Of[PaymentReceivedData]) (uuid.UUID, bool) {
			return evt.Data().OrderID, true
		},
		(*OrderWorkflow).onPaymentReceived,
		PaymentReceived,
	),
	workflow.Reacts(
		func(evt event.Of[PaymentDeclinedData]) (uuid.UUID, bool) {
			return evt.Data().OrderID, true
		},
		(*OrderWorkflow).onPaymentDeclined,
		PaymentDeclined,
	),
	workflow.Compensates(
		func(evt event.Of[StockReleasedData]) (uuid.UUID, bool) {
			return evt.Data().OrderID, true
		},
		(*OrderWorkflow).onStockReleased,
		StockReleased,
	),
	workflow.OnTimeout("payment", (*OrderWorkflow).onPaymentTimeout),
	workflow.OnCompensationTimeout("release-stock", (*OrderWorkflow).onReleaseTimeout),
)
```

### Handlers

Each handler receives a `workflow.Ctx[D]` with the typed event data and returns an error. Handlers orchestrate by combining the methods covered in [Handler Context](#handler-context) — dispatching commands, scheduling or canceling timeouts, and transitioning the workflow's lifecycle. They can also call the workflow's business methods to persist internal state for later use.

```go
func (w *OrderWorkflow) onOrderPlaced(ctx workflow.Ctx[OrderPlacedData]) error {
	data := ctx.Event().Data()
	w.captureOrder(data)

	if err := ctx.Dispatch("reserve-stock", command.New(
		ReserveStockCmd,
		ReserveStockPayload{
			OrderID:     data.OrderID,
			WarehouseID: data.WarehouseID,
			Items:       data.Items,
		},
		command.Aggregate(OrderAggregate, data.OrderID),
	).Any()); err != nil {
		return err
	}

	return ctx.Schedule("payment", time.Now().Add(15*time.Minute))
}

func (w *OrderWorkflow) onPaymentReceived(ctx workflow.Ctx[PaymentReceivedData]) error {
	// Not strictly necessary — Complete() auto-cancels all active timeouts.
	// Shown here to demonstrate explicit timeout cancellation.
	if err := ctx.Unschedule("payment"); err != nil {
		return err
	}

	return ctx.Complete()
}

func (w *OrderWorkflow) onPaymentDeclined(ctx workflow.Ctx[PaymentDeclinedData]) error {
	data := ctx.Event().Data()

	if err := ctx.Compensate(errors.New(data.Reason)); err != nil {
		return err
	}

	w.requestStockRelease(data.Reason)

	if err := ctx.Dispatch("release-stock", command.New(
		ReleaseStockCmd,
		ReleaseStockPayload{
			OrderID:     data.OrderID,
			WarehouseID: w.WarehouseID,
			Reason:      data.Reason,
			Items:       w.Items,
		},
		command.Aggregate(OrderAggregate, data.OrderID),
	).Any()); err != nil {
		return err
	}

	return ctx.Schedule("release-stock", time.Now().Add(30*time.Second))
}

func (w *OrderWorkflow) onStockReleased(ctx workflow.Ctx[StockReleasedData]) error {
	return ctx.Compensated()
}

func (w *OrderWorkflow) onPaymentTimeout(ctx workflow.Ctx[workflow.TimeoutFiredData]) error {
	return ctx.Fail(errors.New("payment timeout"))
}

func (w *OrderWorkflow) onReleaseTimeout(ctx workflow.Ctx[workflow.TimeoutFiredData]) error {
	return ctx.CompensationFailed(errors.New("stock release timeout"))
}
```

### Walkthrough

1. **`onOrderPlaced`** — Calls `captureOrder` first to persist the warehouse ID and line items into the workflow's state (these are needed later if compensation kicks in). Then dispatches `ReserveStockCmd` and schedules the `"payment"` timeout as a 15-minute deadline.
2. **`onPaymentReceived`** — The happy path. Cancels the payment timeout and completes the workflow.
3. **`onPaymentDeclined`** — Enters compensation with `ctx.Compensate(...)`, then calls `requestStockRelease` to record an internal marker event. Next, dispatches `ReleaseStockCmd` using `w.WarehouseID` and `w.Items` — the state that was saved in step 1. Finally, schedules the `"release-stock"` timeout as a safety net.
4. **`onStockReleased`** — Calls `ctx.Compensated()` to mark compensation as complete and end the workflow in `StatusCompensated`. This works because the workflow was moved into `StatusCompensating` in step 3.
5. **`onPaymentTimeout` / `onReleaseTimeout`** — Timeout handlers receive `workflow.TimeoutFiredData` (not a domain event type). `onPaymentTimeout` fails the forward path with `ctx.Fail(...)`, while `onReleaseTimeout` fails the compensation path with `ctx.CompensationFailed(...)`.

## Running the Service

`workflow.NewService` takes a `Config` and one or more workflow definitions and returns a `*Service`. When started, the service:

- Subscribes to trigger events via the [event bus](/guide/event-handlers)
- Persists workflow state and effects via the [event store](/guide/events)
- Runs an **effect runtime** that dispatches pending commands (with automatic retries) and fires due timeouts

### Dependencies

The service needs four dependencies:

- **Event store** (`Config.EventStore`) — persists all workflow events (lifecycle transitions, command requests, timeout scheduling). Also used on startup to recover pending effects, and to share effects between service instances.
- **Event bus** (`Config.EventBus`) — delivers domain events that trigger workflow handlers. If you run multiple instances of the same service, use a [load-balanced](/backends/nats.html#queue-groups-and-load-balancing) subscription so each event is processed by exactly one instance.
- **Command bus** (`Config.CommandBus`) — dispatches commands recorded by `ctx.Dispatch(...)` in handlers.
- **Codec** (`Config.Commands`) — serializes command payloads into the durable command-request events.

### Codec registration

The codec passed as `Config.Commands` must contain the command payload types that handlers dispatch. If it implements `codec.Registerer`, `workflow.NewService` automatically registers the built-in workflow events into it as well — so the simplest setup passes the application's shared registry:

```go
reg := codec.New()
RegisterOrderEvents(reg)   // domain events
RegisterOrderCommands(reg) // command payloads

// Internal workflow events for the workflow's own state.
codec.Register[OrderCapturedData](reg, OrderCaptured)
codec.Register[StockReleaseRequestedData](reg, StockReleaseRequested)

// Built-in workflow events (registered automatically by NewService when the
// codec implements codec.Registerer — explicit registration also works and
// makes the dependency visible).
workflow.RegisterEvents(reg)
```

::: tip Separate event and command registries
If your application keeps separate registries for events and commands, pass the command registry as `Config.Commands` and call `workflow.RegisterEvents(eventRegistry)` on the event registry yourself — the event store and bus need to encode the built-in `goes.workflow.*` events and your internal workflow events.
:::

### Example setup

```go
// Event bus with load balancing so only one instance handles each trigger.
triggerBus := nats.NewEventBus(reg,
	nats.LoadBalancer("order-workflows"),
)

// Event store wrapped with the bus so persisted events are also published.
store := eventstore.WithBus(mongo.NewEventStore(reg), triggerBus)

// Command bus for outgoing commands (separate, non-load-balanced transport).
commandTransport := nats.NewEventBus(reg)
cmdBus := cmdbus.New(reg, commandTransport)

// Create and start the service.
svc := workflow.NewService(workflow.Config{
	EventStore: store,
	EventBus:   triggerBus,
	CommandBus: cmdBus,
	Commands:   reg,
	Strict:     true,

	// Re-process triggers of the last 24h on startup (opt-in; useful with
	// non-durable event buses such as NATS Core).
	TriggerReplayWindow: 24 * time.Hour,
}, OrderProcess)

errs, err := svc.Run(ctx)
if err != nil {
	log.Fatal(err)
}

go func() {
	for err := range errs {
		log.Println(err)
	}
}()
```

`Run` returns a startup error and an asynchronous error channel. Internally it starts the trigger handler (subscribes to the bus) and the effect runtime. Cancel the context to shut everything down.

::: warning Drain the error channel
Always drain the error channel returned by `Run` — an undrained channel eventually pauses the runtime.
:::

### `workflow.Config` {#workflow-config}

| Field | Default | Purpose |
| --- | --- | --- |
| `EventStore` | — | Event store for workflow persistence, effect recovery, and trigger replay |
| `EventBus` | — | Event bus that delivers trigger events |
| `CommandBus` | — | Command bus for dispatching recorded command effects |
| `Commands` | `command.NewRegistry()` | Codec for serializing command payloads in durable effects |
| `NewRepository` | `repository.New(EventStore)` | Constructs the aggregate repository for fetching/saving workflow instances |
| `Strict` | `false` | Report an error when a trigger event correlates to an unknown workflow, instead of silently ignoring it |
| `Workers` | `1` | Number of concurrent trigger workers. Trigger processing is synchronized per workflow, so multiple workers are safe |
| `DispatchInterval` | `100ms` | How often pending commands are dispatched and retried |
| `TimerResolution` | `25ms` | How often the timeout timer checks for expired timeouts |
| `TriggerReplayWindow` | `0` (disabled) | Opt-in startup replay: trigger events no older than the window are re-processed from the event store |
| `ResyncInterval` | `1m` | How often the effect runtime re-reads effect events from the store. Negative disables periodic resyncs |
| `RecoveryWindow` | `30 days` | How far the initial effect recovery reaches back into the store. Negative scans the full history |

## Delivery, Recovery, and Replay

Because workflows are event-sourced, all state — including pending commands and active timeouts — lives in the event stream. This means the runtime can fully reconstruct a workflow's situation from events alone, making recovery after a crash straightforward.

### Live delivery

During normal operation, domain events arrive through the event bus and are routed to the matching workflow handler via the correlator. Each trigger event ID is recorded in the workflow's stream (see [Built-in Events](#built-in-events)), atomically with the handler's changes — so duplicate deliveries are silently ignored and trigger handling is effectively exactly-once.

When saving the workflow fails — typically an optimistic-concurrency conflict with another service instance — the trigger is re-processed against the freshly fetched state a bounded number of times, with deduplication guarding against double handling.

### Trigger replay

Setting `Config.TriggerReplayWindow` replays trigger events no older than the window from the event store on startup. This recovers workflows that missed events while no service was running — useful with non-durable event buses such as [NATS Core](/backends/nats#core-vs-jetstream). The replay runs concurrently with live event processing; trigger deduplication makes the overlap safe, so generous windows are safe too.

Replay is **opt-in**: the zero value disables it. Enable it whenever your event bus doesn't provide durable delivery on its own.

### Effect execution

Recorded effects are executed by the effect runtime, validated against the workflow itself rather than any in-memory state: before a command is dispatched or a timeout fires, the workflow is fetched and must still hold the pending effect. A timeout that was canceled by another service instance therefore never fires, and an already dispatched command is not dispatched again.

Commands are dispatched **at least once**, with a deterministic command ID derived from the effect. There are three crash windows to consider:

1. The process crashes after recording a command request but before dispatching it — the effect runtime picks it up on restart and dispatches it.
2. The process crashes after dispatching a command but before recording it as dispatched — the command is dispatched again with the *same* command ID. The receiving handler should treat duplicate command IDs as no-ops.
3. The process crashes before a scheduled timeout fires — the effect runtime rebuilds the timeout on restart and fires it when due.

In all three cases, the workflow recovers automatically. Deterministic command IDs and aggregate-level idempotency are your safety net — design your command handlers to be safe against duplicate delivery.

### Running multiple instances

Service instances share effects through the event store:

- **At startup**, each instance recovers pending effects from the store, bounded by `Config.RecoveryWindow` (default 30 days). The window must exceed the longest timeout horizon of your workflows plus the maximum expected downtime — effects recorded before the window are not recovered after a restart.
- **At runtime**, periodic resyncs (`Config.ResyncInterval`, default 1m) discover effects recorded by *other* instances — letting any instance take over the pending commands and timeouts of a crashed one — and recover local effect notifications that were dropped under load.
- Effects discovered through resyncs are attempted only once they are older than a short takeover grace (500ms from their recording time), so a healthy instance keeps a head start on its own fresh effects and duplicate dispatches stay rare.

## Snapshots for Long-Lived Workflows

Workflow instances are fetched and saved through an aggregate repository — by default `repository.New(EventStore)`, replaceable via `Config.NewRepository`. A definition can also bring its own typed repository with `workflow.WithRepository`, which is how [snapshots](/guide/snapshots) are enabled for long-lived workflows:

```go
repo := repository.New(store, repository.WithSnapshots(snapshots, snapshot.Every(10)))

var OrderProcess = workflow.Define(
	NewOrderWorkflow,
	workflow.WithRepository(repository.Typed(repo, NewOrderWorkflow)),
	workflow.Starts(workflow.ByAggregateID, (*OrderWorkflow).onOrderPlaced, OrderPlaced),
	// ...
)
```

`workflow.Base` implements snapshot marshaling for its lifecycle and effect state, so workflows without state of their own are snapshot-ready as-is. Workflows that carry their own state must implement `MarshalSnapshot` and `UnmarshalSnapshot` themselves and include the Base state:

```go
func (w *OrderWorkflow) MarshalSnapshot() ([]byte, error) {
	base, err := w.Base.MarshalSnapshot()
	if err != nil {
		return nil, err
	}
	return json.Marshal(orderSnapshot{
		Base:  base,
		Items: w.Items,
	})
}

func (w *OrderWorkflow) UnmarshalSnapshot(p []byte) error {
	var snap orderSnapshot
	if err := json.Unmarshal(p, &snap); err != nil {
		return err
	}
	w.Items = snap.Items
	return w.Base.UnmarshalSnapshot(snap.Base)
}
```

Custom repositories must persist to the same event store the service is configured with: effect recovery and trigger replay read from `Config.EventStore` directly.

## Inspecting Workflow State

Because a workflow instance is an ordinary event-sourced aggregate, answering "where is order X stuck?" is just a repository fetch — for example from an HTTP handler or a CLI tool:

```go
workflows := repository.Typed(repository.New(store), NewOrderWorkflow)

w, err := workflows.Fetch(ctx, orderID)
if err != nil {
	return err
}

w.Status() // e.g. workflow.StatusCompensating
w.Reason() // e.g. "payment declined"
w.Done()   // false
```

Any state the workflow persisted through its own internal events (like `WarehouseID` and `Items` in the [full example](#full-example)) is rebuilt too and can be read directly.

Treat fetched instances as read-only — state transitions belong to the service's trigger handlers. And for dashboards or lists ("all workflows stuck in compensation"), don't fetch instances one by one: build a [projection](/guide/projections) from the built-in lifecycle events described in the [next section](#built-in-events).

## Testing Workflows

`Service.Trigger` processes an event through the same trigger path that is used for events from the event bus — useful for feeding events into the runtime manually in tests, without an event bus round-trip:

```go
svc := workflow.NewService(workflow.Config{
	EventStore: eventstore.New(),
	EventBus:   eventbus.New(),
	CommandBus: cmdBus,
	Commands:   reg,
}, OrderProcess)

errs, _ := svc.Run(ctx)
go func() { for range errs {} }()

err := svc.Trigger(ctx, event.New(
	OrderPlaced,
	OrderPlacedData{...},
	event.Aggregate(orderID, OrderAggregate, 1),
).Any())
```

The [in-memory backends](/backends/in-memory) make full-service integration tests fast: publish domain events on an in-memory bus, then assert on the dispatched commands and the workflow state fetched from the in-memory store.

## Built-in Events

The workflow runtime records its own events automatically — you never create them yourself. They serve two purposes:

1. **Lifecycle observability** — lifecycle events like `goes.workflow.completed` and `goes.workflow.failed` are useful for building [projections](/guide/projections) that track workflow status, or for monitoring and alerting.
2. **Internal bookkeeping** — effect-tracking events are how the effect runtime rebuilds pending commands and active timeouts after a restart.

Call `workflow.RegisterEvents(registry)` to register all built-in event types in a codec — the registry passed as `Config.Commands` receives them [automatically](#codec-registration).

### Lifecycle events

| Event | Data | Purpose |
| --- | --- | --- |
| `goes.workflow.started` | `StartedData` | Workflow instance created |
| `goes.workflow.completed` | `CompletedData` | Workflow finished successfully |
| `goes.workflow.failed` | `FailedData{Reason}` | Workflow failed |
| `goes.workflow.compensation.started` | `CompensationStartedData{Reason}` | Entered compensation |
| `goes.workflow.compensation.completed` | `CompensationCompletedData` | Compensation succeeded |
| `goes.workflow.compensation.failed` | `CompensationFailedData{Reason}` | Compensation failed |

These are particularly useful for projections — for example, a dashboard showing how many workflows are running, completed, or stuck in compensation.

### Effect-tracking events

| Event | Data | Purpose |
| --- | --- | --- |
| `goes.workflow.trigger.recorded` | `TriggerRecordedData{TriggerID, TriggerName}` | Deduplicates trigger events |
| `goes.workflow.command.requested` | `CommandRequestedData{EffectID, Key, ...}` | Durable pending command |
| `goes.workflow.command.dispatched` | `CommandDispatchedData{EffectID}` | Marks command as dispatched |
| `goes.workflow.timeout.requested` | `TimeoutRequestedData{EffectID, Key, At}` | Durable active timeout |
| `goes.workflow.timeout.canceled` | `TimeoutCanceledData{EffectID, Key}` | Cancels an active timeout |
| `goes.workflow.timeout.fired` | `TimeoutFiredData{EffectID, WorkflowID, Key, ScheduledFor}` | Timeout expired |

You typically don't need to subscribe to these directly — they're internal bookkeeping consumed by the workflow runtime itself. But they're available if you need custom monitoring (e.g. alerting on commands that haven't been dispatched within a certain time).

## Production Notes

### Event publication

The service's trigger bus only receives events that are published to it. If you persist domain events via the event store alone, they won't reach the workflow service. Wrap the store with `eventstore.WithBus(store, bus)` so that persisted events are also published to the trigger bus.

### Transport choice

NATS Core works well for the trigger bus **if** you enable `TriggerReplayWindow`, so that startup replay recovers events missed while the process was down. JetStream provides durable bus-level delivery on top of that, which is useful when you want the bus itself to guarantee delivery rather than relying solely on store-based replay.

### Scaling

When running multiple service instances, use a [load-balanced](/backends/nats.html#queue-groups-and-load-balancing) trigger bus so each event is handled by exactly one instance. Keep the command bus transport separate — `cmdbus` should **not** share the load-balanced bus, because command coordination events need to reach the command handlers directly, not be distributed across workflow instances.

Instances coordinate through the event store, not through each other: optimistic concurrency protects workflow state, trigger deduplication prevents double handling, and [periodic resyncs](#running-multiple-instances) let any instance execute the pending effects of a crashed one. To parallelize trigger processing *within* one instance, raise `Config.Workers` — processing is synchronized per workflow, so this is safe.

### Recovery window sizing

`Config.RecoveryWindow` (default 30 days) bounds the startup scan for pending effects. Size it generously: it must cover the longest timeout deadline your workflows schedule, plus however long the service could conceivably be down. A too-small window silently orphans effects that were recorded before it.

## Migrating from the saga Package

The `saga` package — an **in-process** SAGA coordinator that ran a predefined sequence of actions — is deprecated and will be removed in a future release. Because its execution was neither persisted nor distributed, a crashed process could not recover a running SAGA. The `workflow` package replaces it with the durable, event-driven runtime described in this guide.

| `saga` | `workflow` |
| --- | --- |
| `saga.New(...)` (a `saga.Setup`) | `workflow.Define(NewOrderWorkflow, ...)` |
| `saga.Action("step", fn)` | Event-driven handlers: `workflow.Starts(...)`, `workflow.Reacts(...)` |
| `saga.Sequence(...)`, `saga.StartWith(...)` | No fixed sequence — steps advance by reacting to domain events |
| `saga.Compensate("step", "undo-step")` | Explicit compensation phase: `ctx.Compensate(err)`, `workflow.Compensates(...)` handlers, `ctx.Compensated()` |
| `saga.NewExecutor(...).Execute(ctx, setup)` | `workflow.NewService(workflow.Config{...}, defs...)` + `svc.Run(ctx)` |
| `saga.Repository()`, `saga.EventBus()`, `saga.CommandBus()` | `workflow.Config{EventStore, EventBus, CommandBus, Commands}` |
| `action.Context.Dispatch(...)` | `ctx.Dispatch("effect-key", cmd)` — a durable, at-least-once command effect |
| `action.Context.Publish(...)` | Workflows react to events published by your aggregates and command handlers |
| `action.Context.Fetch(...)` | The workflow instance itself is event-sourced state; fetch other aggregates in your command handlers |
| `saga.Report()` / `report.Report` | `Status()` / `Reason()` on the workflow, plus the built-in `goes.workflow.*` events |
| — | Durable timeouts: `ctx.Schedule(...)`, `workflow.OnTimeout(...)` |

The semantics change with the migration:

- A SAGA executed once, synchronously, in the calling process. A workflow is a long-lived, durable instance that is advanced by events — across service restarts and across multiple service instances.
- Control flow is inverted: instead of a central sequence invoking actions, each handler dispatches commands whose resulting domain events trigger the next handler.
- Compensation is not automatic: entering compensation is an explicit decision (`ctx.Compensate`), the undo work is driven by `Compensates` handlers reacting to events, and `ctx.Compensated()` (or `ctx.CompensationFailed()`) ends the phase.
