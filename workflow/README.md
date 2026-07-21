# Workflow

The `workflow` package provides an event-driven, distributed workflow runtime,
also known as a saga or process manager. It replaces the former `saga` package.

## Model

- A workflow instance is an event-sourced aggregate that embeds `*workflow.Base`.
- Workflow behavior is defined statically with `workflow.Define(...)`.
- Domain events start and advance workflows through `workflow.Starts(...)`,
  `workflow.Reacts(...)`, `workflow.Compensates(...)`, `workflow.OnTimeout(...)`,
  and `workflow.OnCompensationTimeout(...)`.
- Trigger events find their workflow instance through a `Correlator`:
  `workflow.ByAggregateID` keys workflows by the aggregate that emitted the
  event, `workflow.ByKey` derives the workflow id from a business key in the
  event payload, and any custom `func(event.Of[Data]) (uuid.UUID, bool)`
  works as well.
- Handlers do not dispatch commands or start timers inline. They record durable
  effects as events on the workflow; a background runtime executes them.
- Compensation is phase-based and explicit. The runtime does not infer reverse
  operations or maintain an automatic rollback stack.

## Lifecycle

A workflow moves through the following statuses:

| Status | Meaning |
| --- | --- |
| `StatusRunning` | Started, not finished. |
| `StatusCompleted` | Finished successfully (terminal). |
| `StatusCompensating` | Undoing previous work after a business failure. |
| `StatusCompensated` | Compensation finished successfully (terminal). |
| `StatusFailed` | Failed directly, or compensation failed (terminal). |

`Base.Reason()` returns why a workflow left the happy path — the reason of the
most recent `Fail`, `Compensate`, or `CompensationFailed` transition.
Entering compensation and every terminal transition cancel all active timeouts.

## Example

```go
type OrderWorkflow struct {
	*workflow.Base
}

func NewOrderWorkflow(id uuid.UUID) *OrderWorkflow {
	return &OrderWorkflow{Base: workflow.New("shop.order_workflow", id)}
}

var OrderProcess = workflow.Define(
	NewOrderWorkflow,
	workflow.Starts(
		workflow.ByAggregateID,
		(*OrderWorkflow).onOrderPlaced,
		OrderPlaced,
	),
	workflow.Reacts(
		workflow.ByAggregateID,
		(*OrderWorkflow).onPaymentReceived,
		PaymentReceived,
	),
	workflow.Reacts(
		workflow.ByAggregateID,
		(*OrderWorkflow).onPaymentDeclined,
		PaymentDeclined,
	),
	workflow.Compensates(
		workflow.ByAggregateID,
		(*OrderWorkflow).onStockReleased,
		StockReleased,
	),
	workflow.OnTimeout("payment", (*OrderWorkflow).onPaymentTimeout),
	workflow.OnCompensationTimeout("release-stock", (*OrderWorkflow).onReleaseTimeout),
)

func (w *OrderWorkflow) onOrderPlaced(ctx workflow.Ctx[OrderPlacedData]) error {
	if err := ctx.Dispatch("reserve-stock", command.New(...).Any()); err != nil {
		return err
	}

	return ctx.Schedule("payment", time.Now().Add(15*time.Minute))
}

func (w *OrderWorkflow) onPaymentReceived(ctx workflow.Ctx[PaymentReceivedData]) error {
	if err := ctx.Unschedule("payment"); err != nil {
		return err
	}

	return ctx.Complete()
}

func (w *OrderWorkflow) onPaymentTimeout(ctx workflow.Ctx[workflow.TimeoutFiredData]) error {
	return ctx.Fail(errors.New("payment timed out"))
}

func (w *OrderWorkflow) onPaymentDeclined(ctx workflow.Ctx[PaymentDeclinedData]) error {
	if err := ctx.Compensate(errors.New("payment declined")); err != nil {
		return err
	}

	if err := ctx.Dispatch("release-stock", command.New(...).Any()); err != nil {
		return err
	}

	return ctx.Schedule("release-stock", time.Now().Add(30*time.Second))
}

func (w *OrderWorkflow) onStockReleased(ctx workflow.Ctx[StockReleasedData]) error {
	return ctx.Compensated()
}

func (w *OrderWorkflow) onReleaseTimeout(ctx workflow.Ctx[workflow.TimeoutFiredData]) error {
	return ctx.CompensationFailed(errors.New("release-stock timed out"))
}
```

Run the service with an event store, event bus, command bus, and one or more
workflow definitions:

```go
svc := workflow.NewService(workflow.Config{
	EventStore: store,
	EventBus:   eventBus,
	CommandBus: commandBus,
	Commands:   registry,
	Strict:     true,

	// Re-process triggers of the last 24h on startup (opt-in; useful with
	// non-durable event buses such as NATS Core).
	TriggerReplayWindow: 24 * time.Hour,
}, OrderProcess)

errs, err := svc.Run(ctx)
```

Always drain the returned error channel; an undrained channel eventually
pauses the runtime.

## Delivery & consistency semantics

- **Triggers are handled exactly once per event id.** Handled trigger ids are
  persisted atomically with the handler's changes, so redelivered or replayed
  events are ignored.
- **Trigger replay is opt-in.** Setting `Config.TriggerReplayWindow` replays
  trigger events no older than the window from the event store on startup,
  recovering workflows that missed events while no service was running —
  useful with non-durable event buses such as NATS Core. The replay runs
  concurrently with live event processing; trigger deduplication makes the
  overlap safe.
- **Commands are dispatched at least once** with a deterministic command id,
  so consumers can deduplicate. A crash between the bus dispatch and
  persisting the dispatch record causes a re-dispatch of the same command.
- **Effect execution is validated against the workflow, not against runtime
  state.** Before a command is dispatched or a timeout fires, the workflow is
  fetched and must still hold the pending effect. A timeout that was canceled
  by another service instance therefore never fires, and an already
  dispatched command is not dispatched again.
- **Service instances share effects through the event store.** Effects
  recorded by one instance become visible to other instances at startup and
  through periodic resyncs (`Config.ResyncInterval`, default 1m, negative
  disables). Effects discovered this way are attempted only once they are
  older than a short takeover grace (500ms from their recording time), so a
  healthy instance keeps a head start on its own fresh effects and duplicate
  dispatches stay rare.
- **Startup recovery is bounded.** The initial effect recovery reads only
  effect events within `Config.RecoveryWindow` (default 30 days, negative
  scans the full history), keeping the startup scan proportional to recent
  activity instead of the total history of the store. The window must exceed
  the longest timeout horizon of the workflows plus the maximum expected
  downtime.
- **Concurrent writes are retried.** When saving a workflow fails — typically
  an optimistic-concurrency conflict with another instance — the trigger is
  re-processed against the fresh state a bounded number of times.

## Repositories & snapshots

Workflow instances are fetched and saved through an aggregate repository —
by default `repository.New(EventStore)`, replaceable via
`Config.NewRepository`. A definition can also bring its own typed repository
with `workflow.WithRepository`, which is how snapshots are enabled for
long-lived workflows:

```go
repo := repository.New(store, repository.WithSnapshots(snapshots, snapshot.Every(10)))

var OrderProcess = workflow.Define(
	NewOrderWorkflow,
	workflow.WithRepository(repository.Typed(repo, NewOrderWorkflow)),
	workflow.Starts(workflow.ByAggregateID, (*OrderWorkflow).onOrderPlaced, OrderPlaced),
	// ...
)
```

`workflow.Base` implements snapshot marshaling for its lifecycle and effect
state, so workflows without state of their own are snapshot-ready as-is.
Workflows that carry their own state must implement `MarshalSnapshot` and
`UnmarshalSnapshot` themselves and include the Base state (see the
documentation of `Base.MarshalSnapshot`).

Custom repositories must persist to the same event store the Service is
configured with: effect recovery and trigger replay read from
`Config.EventStore` directly.

## Built-in events

The package persists these built-in workflow events:

- `goes.workflow.started`
- `goes.workflow.completed`
- `goes.workflow.failed`
- `goes.workflow.compensation.started`
- `goes.workflow.compensation.completed`
- `goes.workflow.compensation.failed`
- `goes.workflow.trigger.recorded`
- `goes.workflow.command.requested`
- `goes.workflow.command.dispatched`
- `goes.workflow.timeout.requested`
- `goes.workflow.timeout.canceled`
- `goes.workflow.timeout.fired`

Register them with `workflow.RegisterEvents(registry)` when a codec-backed
transport or store needs to encode/decode workflow event payloads. The
registry passed as `Config.Commands` receives them automatically.

## Migrating from package saga

TODO
