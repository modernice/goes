# Saga

The `saga` package provides an event-driven, distributed saga/process-manager runtime.

## Model

- A saga instance embeds `*saga.Base`.
- Saga behavior is defined statically with `saga.Define(...)`.
- Domain events start or advance the saga through `saga.Starts(...)`, `saga.Reacts(...)`, `saga.Compensates(...)`, `saga.OnTimeout(...)`, and `saga.OnCompensationTimeout(...)`.
- Handlers do not dispatch commands or timers inline. They record durable saga events.
- Compensation is phase-based and explicit. The runtime does not infer reverse operations or maintain an automatic rollback stack.
- Background workers replay pending command and timeout effects from the event store.
- On startup, trigger events are also replayed from the event store before the runtime continues live processing. This allows recovery with non-durable event buses such as NATS Core.
- Trigger replay is bounded by `saga.Config.TriggerReplayWindow`; the default is `24*time.Hour`.

## Example

```go
type OrderSaga struct {
	*saga.Base
}

func NewOrderSaga(id uuid.UUID) *OrderSaga {
	s := &OrderSaga{Base: saga.New("shop.order_saga", id)}

	event.ApplyWith(s, s.started, saga.Started)
	return s
}

var OrderProcess = saga.Define(
	NewOrderSaga,
	saga.Starts(
		saga.AggregateID[OrderPlacedData](),
		(*OrderSaga).onOrderPlaced,
		OrderPlaced,
	),
	saga.Reacts(
		saga.AggregateID[PaymentReceivedData](),
		(*OrderSaga).onPaymentReceived,
		PaymentReceived,
	),
	saga.Compensates(
		saga.AggregateID[StockReleasedData](),
		(*OrderSaga).onStockReleased,
		StockReleased,
	),
	saga.OnTimeout("payment", (*OrderSaga).onPaymentTimeout),
	saga.OnCompensationTimeout("release-stock", (*OrderSaga).onReleaseTimeout),
)

func (s *OrderSaga) onOrderPlaced(ctx saga.Ctx[OrderPlacedData]) error {
	if err := ctx.Dispatch("reserve-stock", command.New(...).Any()); err != nil {
		return err
	}

	return ctx.Schedule("payment", time.Now().Add(15*time.Minute))
}

func (s *OrderSaga) onPaymentReceived(ctx saga.Ctx[PaymentReceivedData]) error {
	if err := ctx.CancelTimeout("payment"); err != nil {
		return err
	}

	return ctx.Complete()
}

func (s *OrderSaga) onPaymentDeclined(ctx saga.Ctx[PaymentDeclinedData]) error {
	if err := ctx.Compensate(errors.New("payment declined")); err != nil {
		return err
	}

	if err := ctx.Dispatch("release-stock", command.New(...).Any()); err != nil {
		return err
	}

	return ctx.Schedule("release-stock", time.Now().Add(30*time.Second))
}

func (s *OrderSaga) onStockReleased(ctx saga.Ctx[StockReleasedData]) error {
	return ctx.Compensated()
}
```

Run the service with an event store, event bus, command bus, and one or more saga definitions:

```go
svc := saga.NewService(saga.Config{
	Encoding: registry,
	Store:    store,
	Bus:      eventBus,
	Commands: commandBus,
	Strict:   true,
}, OrderProcess)

errs, err := svc.Run(ctx)
```

## Built-in Events

The package persists these built-in saga events:

- `goes.saga.started`
- `goes.saga.completed`
- `goes.saga.failed`
- `goes.saga.compensation.started`
- `goes.saga.compensation.completed`
- `goes.saga.compensation.failed`
- `goes.saga.trigger.recorded`
- `goes.saga.command.requested`
- `goes.saga.command.dispatched`
- `goes.saga.timeout.requested`
- `goes.saga.timeout.canceled`
- `goes.saga.timeout.fired`

Register them with `saga.RegisterEvents(registry)` when a codec-backed transport or
store needs to encode/decode saga event payloads.
