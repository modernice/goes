# Sagas

Sagas coordinate multi-step workflows that unfold over time. When a business process spans multiple events, commands, timeouts, and compensation steps — potentially across aggregates or services — a saga orchestrates the sequence durably and recovers from failures.

> See also: [Commands](/guide/commands) for single-step dispatch, [Aggregates](/guide/aggregates) for consistency boundaries, [Event Handlers](/guide/event-handlers) for lightweight observers.

## When to Use a Saga

A saga is a good fit when the workflow:

- waits for future events before deciding the next step
- sets deadlines (timeouts) that trigger fallback behavior
- crosses service or process boundaries
- needs explicit compensation to undo partial progress

You don't need a saga when the entire rule fits inside a single [aggregate](/guide/aggregates) consistency boundary, or when a single [command handler](/guide/commands) can execute the whole operation immediately. A saga coordinates *between* aggregates and services — the aggregates still own their own state and invariants, while the saga drives the overall workflow forward.

## Mental Model

If you've worked with [aggregates](/guide/aggregates), sagas will feel familiar — under the hood, a saga instance *is* an event-sourced aggregate:

- It embeds `*saga.Base`.
- External domain events start or advance it.
- Handlers never perform side effects directly — they record durable saga events instead.
- Background workers later dispatch pending commands and fire due timeouts.
- State, pending commands, and active timeouts survive restarts because they are rebuilt from the event store.

Saga effects are **at-least-once**. Commands and timeout-fired events may be retried after a crash, so downstream handlers and aggregates should be idempotent.

## Defining a Saga

A saga definition has three parts: a struct that holds the saga's state, a constructor, and a set of registrations that tell the runtime which events to listen for and how to handle them.

### `saga.Base` and `saga.New`

Embed `*saga.Base` in your saga struct. The constructor calls `saga.New` with the aggregate name and ID:

```go
type OrderSaga struct {
	*saga.Base

	WarehouseID uuid.UUID
	Items       []LineItem
}

func NewOrderSaga(id uuid.UUID) *OrderSaga {
	s := &OrderSaga{
		Base: saga.New("shop.order_saga", id),
	}

	// Register appliers for internal saga events (shown in the full example below).
	event.ApplyWith(s, s.orderCaptured, OrderCaptured)
	event.ApplyWith(s, s.stockReleaseRequested, StockReleaseRequested)

	return s
}
```

`Base` provides the event-sourced lifecycle state, trigger deduplication, pending command effects, and active timeout tracking. If the saga needs its own state beyond what `Base` tracks, persist it through internal events and register appliers with `event.ApplyWith` — the same way you would in an [aggregate](/guide/aggregates). The [full example](#full-example) below shows this in detail.

### `saga.Define`

`saga.Define` ties a typed constructor to one or more registrations. This is the unit you pass to `saga.NewService(...)`:

```go
var OrderProcess = saga.Define(
	NewOrderSaga,
	saga.Starts(...),
	saga.Reacts(...),
	saga.Compensates(...),
	saga.OnTimeout(...),
	saga.OnCompensationTimeout(...),
)
```

### Registration Functions

Each registration takes a **correlator**, a **handler**, and one or more **event names**:

| Function | Phase | Purpose |
| --- | --- | --- |
| `saga.Starts(correlator, handler, events...)` | Forward | Creates a new saga instance if one doesn't exist for the correlated ID |
| `saga.Reacts(correlator, handler, events...)` | Forward | Advances the saga during normal progression |
| `saga.Compensates(correlator, handler, events...)` | Compensation | Runs only after the saga entered compensation via `ctx.Compensate(...)` |
| `saga.OnTimeout(key, handler)` | Forward | Fires when a forward-phase timeout expires |
| `saga.OnCompensationTimeout(key, handler)` | Compensation | Fires when a compensation-phase timeout expires |

## Correlators

When a domain event arrives, the saga runtime needs to figure out *which saga instance* should handle it. That's the correlator's job — it inspects the event and returns the UUID of the saga instance it belongs to.

Every `Starts`, `Reacts`, and `Compensates` registration requires a correlator. The most common case is using the aggregate ID of the triggering event:

```go
saga.Starts(
	saga.AggregateID[OrderPlacedData](),
	(*OrderSaga).onOrderPlaced,
	OrderPlaced,
)
```

`saga.AggregateID[T]()` extracts the aggregate ID from the event and uses it as the saga instance ID.

For workflows where the saga ID comes from a field in the event data, write a custom correlator:

```go
saga.Reacts(
	func(evt event.Of[PaymentReceivedData]) (uuid.UUID, bool) {
		return evt.Data().OrderID, true
	},
	(*OrderSaga).onPaymentReceived,
	PaymentReceived,
)
```

Return `false` as the second value to skip the event.

In practice, cross-aggregate events (like `PaymentReceived` from the payment aggregate) should include the correlation ID (e.g. `OrderID`) in their event data — this is the simplest and most common approach. When the producing aggregate genuinely doesn't know about the saga's domain (e.g. a generic inventory service), the correlator can use an injected service or lookup to resolve the saga ID through more indirection.

## Handler Context

Every saga handler receives a `saga.Ctx[D]` — the main interface for interacting with the saga runtime. It embeds `context.Context` and exposes the triggering event via `ctx.Event()`, along with methods to record effects and transition the saga's lifecycle.

### Recording effects

These methods record durable saga events — they don't execute anything immediately. Background workers pick up pending commands and fire due timeouts later.

#### `Dispatch(step, cmd)`

Queues a [command](/guide/commands) for background dispatch. The `step` key identifies the command within the current trigger — the runtime uses it together with the saga ID and trigger event ID to compute a deterministic effect ID, so calling `Dispatch` with the same step and trigger is idempotent.

The command must be passed as a `command.Command` (i.e. `command.Of[any]`). Since `command.New` returns a typed `command.Cmd[P]`, call `.Any()` to erase the type parameter:

```go
ctx.Dispatch("reserve-stock", command.New(
	ReserveStockCmd,
	ReserveStockPayload{OrderID: orderID},
	command.Aggregate(OrderAggregate, orderID),
).Any())
```

The saga serializes the command payload immediately using the codec from `saga.Config.Encoding` (shown in [Running the Service](#running-the-service) below), so the payload type must be registered in that codec. The actual dispatch happens later in a background worker, with automatic retries on failure (controlled by `saga.Config.RetryInterval`).

#### `Schedule(timeout, at)`

Sets a named deadline. When the clock reaches `at`, the timeout worker fires the timeout and the corresponding `OnTimeout` (or `OnCompensationTimeout`) handler runs.

```go
ctx.Schedule("payment", time.Now().Add(15*time.Minute))
```

If a timeout with the same key already exists, `Schedule` automatically cancels the old one and replaces it with the new deadline. If the key and time are identical to the existing timeout, it's a no-op. The polling frequency of the timeout worker is controlled by `saga.Config.TimerResolution`.

#### `CancelTimeout(timeout)`

Cancels an active timeout by key. If no timeout with that key exists, it's a no-op.

```go
ctx.CancelTimeout("payment")
```

### Transitioning the saga

These methods move the saga through its lifecycle. Each one also cancels all active timeouts before transitioning.

| Method | From | To | Description |
| --- | --- | --- | --- |
| `Complete()` | Running | Completed | The workflow finished successfully. |
| `Fail(err)` | Running | Failed | The workflow failed. No compensation. |
| `Compensate(err)` | Running | Compensating | Something went wrong — begin undoing side effects. |
| `Compensated()` | Compensating | Failed | Compensation succeeded. The saga is over. |
| `CompensationFailed(err)` | Compensating | Failed | Compensation itself failed. Needs manual intervention. |

Calling a transition from the wrong status returns an error (`ErrInvalidTransition` or `ErrNotCompensating`). Calling `Complete()` on an already-completed saga, or `Fail()` on an already-failed saga, is a safe no-op.

### Why does `Compensated()` end in Failed?

Compensation means the original workflow didn't succeed — the saga is recovering, not completing. Both `Compensated()` and `CompensationFailed()` end in `StatusFailed` because the business goal was not achieved either way. The difference is recorded in compensation metadata:

| Outcome | Status | `Compensation()` |
| --- | --- | --- |
| Saga failed, no compensation | `StatusFailed` | zero-value |
| Saga failed, compensation succeeded | `StatusFailed` | `Completed: true` |
| Saga failed, compensation also failed | `StatusFailed` | `Failed: true, Error: "..."` |

Query `Base.Status()`, `Base.FailureReason()`, and `Base.Compensation()` to distinguish between these outcomes.

## Full Example

This example models an order workflow with forward progression and compensation:

- `OrderPlaced` starts the saga, dispatches a stock reservation, and sets a 15-minute payment deadline
- `PaymentReceived` cancels the timeout and completes the saga
- `PaymentDeclined` enters compensation and dispatches a stock release
- `StockReleased` finishes compensation
- Timeouts fail their respective phases

### Domain types

```go
const OrderSagaAggregate = "shop.order_saga"
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

// Internal saga events for rebuilding state.
const (
	OrderCaptured         = "shop.order_saga.order_captured"
	StockReleaseRequested = "shop.order_saga.stock_release_requested"
)
```

Each event and command name has a corresponding payload struct (e.g. `OrderPlacedData`, `ReserveStockPayload`). The handlers below access these through `ctx.Event().Data()`. The payload structs themselves are straightforward data carriers, so they're omitted here for brevity.

### Saga struct, constructor, and state methods

```go
type OrderSaga struct {
	*saga.Base

	WarehouseID         uuid.UUID
	Items               []LineItem
	StockReleasePending bool
	StockReleaseReason  string
}

func NewOrderSaga(id uuid.UUID) *OrderSaga {
	s := &OrderSaga{
		Base: saga.New(OrderSagaAggregate, id),
	}

	event.ApplyWith(s, s.orderCaptured, OrderCaptured)
	event.ApplyWith(s, s.stockReleaseRequested, StockReleaseRequested)

	return s
}

// captureOrder records the order data so compensation handlers can reference it later.
func (s *OrderSaga) captureOrder(data OrderPlacedData) {
	aggregate.Next(s, OrderCaptured, OrderCapturedData{
		WarehouseID: data.WarehouseID,
		Items:       data.Items,
	})
}

func (s *OrderSaga) orderCaptured(evt event.Of[OrderCapturedData]) {
	s.WarehouseID = evt.Data().WarehouseID
	s.Items = evt.Data().Items
}

// requestStockRelease marks that a stock release is pending.
func (s *OrderSaga) requestStockRelease(reason string) {
	aggregate.Next(s, StockReleaseRequested, StockReleaseRequestedData{
		Reason: reason,
	})
}

func (s *OrderSaga) stockReleaseRequested(evt event.Of[StockReleaseRequestedData]) {
	s.StockReleasePending = true
	s.StockReleaseReason = evt.Data().Reason
}
```

Each business method (e.g. `captureOrder`) records an internal saga event with `aggregate.Next`, and the corresponding applier (e.g. `orderCaptured`) updates the saga's in-memory state when that event is replayed. This is the same pattern used in [aggregates](/guide/aggregates) — the internal events persist state durably so it survives restarts.

### Definition

This is where the event wiring comes together. The definition binds each domain event to a correlator and handler, and registers timeout handlers by key. Only `OrderPlaced` uses `saga.AggregateID` because it originates from the order aggregate — its aggregate ID *is* the order ID. The other events come from the payment and stock aggregates, so they use custom correlators that extract the `OrderID` from the event data.

```go
var OrderProcess = saga.Define(
	NewOrderSaga,
	saga.Starts(
		saga.AggregateID[OrderPlacedData](),
		(*OrderSaga).onOrderPlaced,
		OrderPlaced,
	),
	saga.Reacts(
		func(evt event.Of[PaymentReceivedData]) (uuid.UUID, bool) {
			return evt.Data().OrderID, true
		},
		(*OrderSaga).onPaymentReceived,
		PaymentReceived,
	),
	saga.Reacts(
		func(evt event.Of[PaymentDeclinedData]) (uuid.UUID, bool) {
			return evt.Data().OrderID, true
		},
		(*OrderSaga).onPaymentDeclined,
		PaymentDeclined,
	),
	saga.Compensates(
		func(evt event.Of[StockReleasedData]) (uuid.UUID, bool) {
			return evt.Data().OrderID, true
		},
		(*OrderSaga).onStockReleased,
		StockReleased,
	),
	saga.OnTimeout("payment", (*OrderSaga).onPaymentTimeout),
	saga.OnCompensationTimeout("release-stock", (*OrderSaga).onReleaseTimeout),
)
```

### Handlers

Each handler receives a `saga.Ctx[D]` with the typed event data and returns an error. Handlers orchestrate by combining the methods covered in [Handler Context](#handler-context) — dispatching commands, scheduling or canceling timeouts, and transitioning the saga's lifecycle. They can also call the saga's business methods to persist internal state for later use.

```go
func (s *OrderSaga) onOrderPlaced(ctx saga.Ctx[OrderPlacedData]) error {
	data := ctx.Event().Data()
	s.captureOrder(data)

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

func (s *OrderSaga) onPaymentReceived(ctx saga.Ctx[PaymentReceivedData]) error {
	// Not strictly necessary — Complete() auto-cancels all active timeouts.
	// Shown here to demonstrate explicit timeout cancellation.
	if err := ctx.CancelTimeout("payment"); err != nil {
		return err
	}

	return ctx.Complete()
}

func (s *OrderSaga) onPaymentDeclined(ctx saga.Ctx[PaymentDeclinedData]) error {
	data := ctx.Event().Data()

	if err := ctx.Compensate(errors.New(data.Reason)); err != nil {
		return err
	}

	s.requestStockRelease(data.Reason)

	if err := ctx.Dispatch("release-stock", command.New(
		ReleaseStockCmd,
		ReleaseStockPayload{
			OrderID:     data.OrderID,
			WarehouseID: s.WarehouseID,
			Reason:      data.Reason,
			Items:       s.Items,
		},
		command.Aggregate(OrderAggregate, data.OrderID),
	).Any()); err != nil {
		return err
	}

	return ctx.Schedule("release-stock", time.Now().Add(30*time.Second))
}

func (s *OrderSaga) onStockReleased(ctx saga.Ctx[StockReleasedData]) error {
	return ctx.Compensated()
}

func (s *OrderSaga) onPaymentTimeout(ctx saga.Ctx[saga.TimeoutFiredData]) error {
	return ctx.Fail(errors.New("payment timeout"))
}

func (s *OrderSaga) onReleaseTimeout(ctx saga.Ctx[saga.TimeoutFiredData]) error {
	return ctx.CompensationFailed(errors.New("stock release timeout"))
}
```

### Walkthrough

1. **`onOrderPlaced`** — Calls `captureOrder` first to persist the warehouse ID and line items into the saga's state (these are needed later if compensation kicks in). Then dispatches `ReserveStockCmd` and schedules the `"payment"` timeout as a 15-minute deadline.
2. **`onPaymentReceived`** — The happy path. Cancels the payment timeout and completes the saga.
3. **`onPaymentDeclined`** — Enters compensation with `ctx.Compensate(...)`, then calls `requestStockRelease` to record an internal marker event. Next, dispatches `ReleaseStockCmd` using `s.WarehouseID` and `s.Items` — the state that was saved in step 1. Finally, schedules the `"release-stock"` timeout as a safety net.
4. **`onStockReleased`** — Calls `ctx.Compensated()` to mark compensation as complete. This works because the saga was moved into `StatusCompensating` in step 3.
5. **`onPaymentTimeout` / `onReleaseTimeout`** — Timeout handlers receive `saga.TimeoutFiredData` (not a domain event type). `onPaymentTimeout` fails the forward path with `ctx.Fail(...)`, while `onReleaseTimeout` fails the compensation path with `ctx.CompensationFailed(...)`.

## Running the Service

`saga.NewService` takes a `Config` and one or more saga definitions and returns a `*Service`. When started, the service:

- Subscribes to trigger events via the [event bus](/guide/event-handlers)
- Persists saga state and effects via the [event store](/guide/events)
- Runs a **command worker** that dispatches pending commands (with automatic retries)
- Runs a **timeout worker** that fires expired timeouts

### Dependencies

The service needs four dependencies:

- **Event store** (`event.Store`) — persists all saga events (lifecycle transitions, command requests, timeout scheduling). Also used on startup to recover pending work.
- **Event bus** (`event.Bus`) — delivers domain events that trigger saga handlers. If you run multiple instances of the same service, use a [load-balanced](/backends/nats.html#queue-groups-and-load-balancing) subscription so each event is processed by exactly one instance.
- **Command bus** (`command.Bus`) — dispatches commands queued by `ctx.Dispatch(...)` in handlers.
- **Codec** (`codec.Encoding`) — serializes command payloads into the durable command-request events. This is typically a separate [codec registry](/guide/codec) from the event registry.

### Codec registration

You need two codec registries — one for events, one for commands:

- **Event registry**: Register your domain events, the saga's internal events (e.g. `OrderCapturedData`), and the built-in saga lifecycle events via `saga.RegisterEvents(registry)`.
- **Command registry**: Register command payload types. Pass this as `Config.Encoding` so the saga can serialize command effects.

::: tip
If the codec implements `codec.Registerer`, `saga.NewService` auto-calls `saga.RegisterEvents` — but registering explicitly makes the dependency visible.
:::

### Example setup

```go
// Event codec — domain events, internal saga events, and saga lifecycle events.
eventReg := codec.New()
RegisterOrderEvents(eventReg)
codec.Register[OrderCapturedData](eventReg, OrderCaptured)
codec.Register[StockReleaseRequestedData](eventReg, StockReleaseRequested)
saga.RegisterEvents(eventReg)

// Command codec — command payloads dispatched by the saga.
cmdReg := codec.New()
RegisterOrderCommands(cmdReg)

// Event bus with load balancing so only one instance handles each trigger.
triggerBus := nats.NewEventBus(eventReg,
	nats.LoadBalancer("order-sagas"),
)

// Event store wrapped with the bus so persisted events are also published.
store := eventstore.WithBus(eventstore.New(), triggerBus)

// Command bus for outgoing commands.
commandTransport := nats.NewEventBus(eventReg)
cmdBus := cmdbus.New[int](cmdReg, commandTransport)

// Create and start the service.
svc := saga.NewService(saga.Config{
	Encoding: cmdReg,
	Store:    store,
	Bus:      triggerBus,
	Commands: cmdBus,
	Strict:   true,
}, OrderProcess)

errs, err := svc.Run(ctx)
```

`Run` returns a startup error and an asynchronous error channel. Internally it starts the event handler (subscribes to the bus), the command worker, and the timeout worker. Cancel the context to shut everything down.

### `saga.Config`

| Field | Default | Purpose |
| --- | --- | --- |
| `Encoding` | `command.NewRegistry()` | Codec for serializing command payloads in durable effects |
| `Store` | — | Event store for saga persistence and startup recovery |
| `Bus` | — | Event bus for trigger event delivery |
| `Commands` | — | Command bus for dispatching outgoing commands |
| `Strict` | `false` | Return error when a non-start trigger correlates to a non-existent saga |
| `RetryInterval` | `100ms` | How often the command worker retries failed dispatches |
| `TimerResolution` | `25ms` | How often the timeout worker checks for expired timeouts |
| `TriggerReplayWindow` | `24h` | How far back to replay trigger events from the store on startup. `0` uses the default. A negative value disables replay. |

## Delivery, Recovery, and Replay

Because sagas are event-sourced, all state — including pending commands and active timeouts — lives in the event stream. This means the runtime can fully reconstruct a saga's situation from events alone, making recovery after a crash straightforward.

### Live delivery

During normal operation, domain events arrive through the event bus and are routed to the matching saga handler via the correlator. Each trigger event ID is recorded in the saga's stream (see [Built-in Events](#built-in-events)), so duplicate deliveries are silently ignored.

### Startup recovery

When the service starts (or restarts), it does three things:

1. **Replays recent triggers** from the event store, bounded by `TriggerReplayWindow` (default 24h). This catches any events that arrived while the process was down. Already-seen trigger IDs are skipped.
2. **Rebuilds pending commands** from the saga's event stream — commands that were recorded but not yet dispatched are picked up by the command worker.
3. **Rebuilds active timeouts** from the saga's event stream — timeouts that were scheduled but not yet fired are picked up by the timeout worker.

### Crash safety

The effect model is intentionally **at-least-once**. There are three crash windows to consider:

1. The process crashes after recording a command request but before dispatching it — the command worker picks it up on restart and dispatches it.
2. The process crashes after dispatching a command but before recording it as dispatched — the command may be dispatched again with the same deterministic command ID. The receiving aggregate should treat duplicate command IDs as no-ops.
3. The process crashes before a scheduled timeout fires — the timeout worker rebuilds the timeout on restart and fires it when due.

In all three cases, the saga recovers automatically. Deterministic command IDs and aggregate-level idempotency are your safety net — design your command handlers to be safe against duplicate delivery.

## Built-in Events

The saga runtime records its own events automatically — you never create them yourself. They serve two purposes:

1. **Lifecycle observability** — lifecycle events like `goes.saga.completed` and `goes.saga.failed` are useful for building [projections](/guide/projections) that track saga status, or for monitoring and alerting.
2. **Internal bookkeeping** — effect-tracking events are how the command and timeout workers rebuild their state on startup (see [Startup recovery](#startup-recovery) above).

Call `saga.RegisterEvents(registry)` to register all built-in event types in your codec.

### Lifecycle events

| Event | Data | Purpose |
| --- | --- | --- |
| `goes.saga.started` | `StartedData` | Saga instance created |
| `goes.saga.completed` | `CompletedData` | Saga finished successfully |
| `goes.saga.failed` | `FailedData{Error}` | Saga failed |
| `goes.saga.compensation.started` | `CompensationStartedData{Reason}` | Entered compensation mode |
| `goes.saga.compensation.completed` | `CompensationCompletedData` | Compensation succeeded |
| `goes.saga.compensation.failed` | `CompensationFailedData{Error}` | Compensation failed |

These are particularly useful for projections — for example, a dashboard showing how many sagas are running, completed, or stuck in compensation.

### Effect-tracking events

| Event | Data | Purpose |
| --- | --- | --- |
| `goes.saga.trigger.recorded` | `TriggerRecordedData{TriggerID, TriggerName}` | Deduplicates trigger events |
| `goes.saga.command.requested` | `CommandRequestedData{EffectID, Name, ...}` | Durable pending command |
| `goes.saga.command.dispatched` | `CommandDispatchedData{EffectID}` | Marks command as sent |
| `goes.saga.timeout.requested` | `TimeoutRequestedData{EffectID, Key, At}` | Durable pending timeout |
| `goes.saga.timeout.canceled` | `TimeoutCanceledData{EffectID, Key}` | Cancels an active timeout |
| `goes.saga.timeout.fired` | `TimeoutFiredData{EffectID, SagaID, Key}` | Timeout expired |

You typically don't need to subscribe to these directly — they're internal bookkeeping consumed by the saga runtime itself. But they're available if you need custom monitoring (e.g. alerting on commands that haven't been dispatched within a certain time).

## Production Notes

### Event publication

The saga's trigger bus only receives events that are published to it. If you persist domain events via `eventstore.New()` alone, they won't reach the saga. Wrap the store with `eventstore.WithBus(store, bus)` so that persisted events are also published to the trigger bus.

### Transport choice

NATS Core works well for the trigger bus because startup replay (bounded by `TriggerReplayWindow`) recovers any events missed while the process was down. JetStream provides durable bus-level delivery on top of that, which is useful when you want the bus itself to guarantee delivery rather than relying solely on store-based replay.

### Scaling

When running multiple saga service instances, use a [load-balanced](/backends/nats.html#queue-groups-and-load-balancing) trigger bus so each event is handled by exactly one instance. Keep the command bus transport separate — `cmdbus` should **not** share the load-balanced bus, because command dispatch events need to reach the command handler directly, not be distributed across saga instances.
