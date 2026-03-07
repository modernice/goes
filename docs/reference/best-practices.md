# Best Practices

Patterns and guidelines for building maintainable event-sourced applications with goes.

## Aggregate Design

**Keep aggregates small.** An aggregate is a consistency boundary, not an entity container. If two pieces of state don't need to be consistent within the same transaction, they belong in separate aggregates. A `Product` and an `Order` should be separate aggregates even if they're related — but they can share DTOs. For example, an `Order` aggregate might use `ProductDTO` to store product snapshots in its line items:

```go
type LineItem struct {
	Product  ProductDTO
	Quantity int
}
```

`ProductDTO` is defined once (alongside the `Product` aggregate) and reused wherever product data needs to travel across aggregate boundaries.

When a single aggregate grows too large, consider [splitting it](/guide/aggregate-splitting) into multiple aggregates that share the same UUID — one per concern.

**Validate in business methods, not in appliers.** Event appliers replay from history — if an applier returns an error or panics, the aggregate can never be reconstructed. Put all validation in the public method that calls `aggregate.Next`:

```go
// Good: validate before raising
func (p *Product) SetPrice(price int) error {
	if price < 0 {
		return fmt.Errorf("price must be positive")
	}
	aggregate.Next(p, PriceChanged, PriceChangedData{Price: price})
	return nil
}

// Bad: validation in applier — breaks replay
func (p *Product) applyPriceChanged(evt event.Of[PriceChangedData]) {
	if evt.Data().Price < 0 {
		panic("invalid price") // This breaks reconstruction
	}
	p.Price = evt.Data().Price
}
```

**Skip no-op events.** If calling `SetPrice(2999)` when the price is already 2999, don't raise an event — nothing changed:

```go
func (p *Product) SetPrice(price int) error {
	if p.Price == price {
		return nil // No change, no event
	}
	aggregate.Next(p, PriceChanged, PriceChangedData{Price: price})
	return nil
}
```

Not every event needs to carry data or change state, though. Sometimes an event just signals that something happened — a `HealthChecked` or `ImportTriggered` event, for example. Use `struct{}` as the event data for these signals:

```go
const HealthChecked = "system.health_checked"

aggregate.Next(s, HealthChecked, struct{}{})
```

**Accept value objects, not their fields.** When a business method takes structured data, pass the value object directly instead of decomposing it into individual parameters. Put validation on the value object itself with a `Validate()` method:

```go
// Good: accept the value object, let it validate itself
func (d Discount) Validate() error {
	if d.Label == "" {
		return fmt.Errorf("discount label is required")
	}
	if d.Percent <= 0 || d.Percent > 100 {
		return fmt.Errorf("discount percent must be between 1 and 100")
	}
	return nil
}

func (p *Pricing) AddDiscount(d Discount) error {
	if err := d.Validate(); err != nil {
		return err
	}
	aggregate.Next(p, DiscountAdded, d)
	return nil
}

// Bad: decompose into primitives — loses structure, duplicates validation
func (p *Pricing) AddDiscount(label string, percent int) error {
	// ...
}
```

This keeps validation close to the data it validates, makes the value object reusable across aggregates, and simplifies command handlers since the payload can be passed straight through.

**Return errors from methods, not from constructors.** The constructor (`NewProduct(id)`) should only set up the aggregate base and register event handlers. All business logic — including initial creation — belongs in a separate method:

```go
p := shop.NewProduct(uuid.New())  // Just sets up handlers
err := p.Create("Mouse", 2999)    // Business logic here
```

## Event Design

**Name events in past tense.** Events describe something that already happened:

```go
// Good
const ProductCreated = "shop.product.created"
const PriceChanged  = "shop.product.price_changed"

// Bad
const CreateProduct = "shop.product.create"
const ChangePrice   = "shop.product.change_price"
```

**Include all data needed to reconstruct state.** The applier should not need to look anything up — everything it needs should be in the event data:

```go
// Good: self-contained
type PriceChangedData struct {
	Price    int
	Currency string
}

// Bad: requires lookup
type PriceChangedData struct {
	PriceID string // What price? Need to look it up
}
```

**Prefer fat events.** Include context that consumers will need — not just the new value, but the old value too. A projection that builds a price history or a notification that says "price dropped from X to Y" shouldn't have to reconstruct the previous state:

```go
// Good: consumers can react to the change without looking up previous state
type PriceChangedData struct {
	OldPrice int
	NewPrice int
	Currency string
}

// Lean: what was the price before? No way to tell without replaying history
type PriceChangedData struct {
	NewPrice int
}
```

The storage cost is negligible. The DX improvement is not.

**Use domain-specific names.** Namespace events by domain and aggregate:

```go
const ProductCreated = "shop.product.created"   // Good
const EntityUpdated  = "entity.updated"          // Bad: too generic
```

**Colocate codec registrations with the aggregate.** Put the registration functions in the same file as the aggregate and its events. Keep events and commands in separate functions — they use separate codec registries:

```go
// In product.go, alongside the aggregate and its events
func RegisterProductEvents(r codec.Registerer) {
	codec.Register[ProductCreatedData](r, ProductCreated)
	codec.Register[PriceChangedData](r, PriceChanged)
	codec.Register[StockAdjustedData](r, StockAdjusted)
}

func RegisterProductCommands(r codec.Registerer) {
	codec.Register[CreateProductPayload](r, CreateProductCmd)
	codec.Register[string](r, RenameProductCmd)
	codec.Register[int](r, ChangePriceCmd)
}
```

## Command Design

**Commands express intent, events express outcome.** A command says "please do this," an event says "this happened." They're not interchangeable:

```go
// Command: intent
cmd := command.New(CreateProduct, CreateProductPayload{Name: "Mouse", Price: 2999})

// Event: outcome
aggregate.Next(p, ProductCreated, ProductCreatedData{Name: "Mouse", Price: 2999})
```

**One command, one handler.** Each command is processed by a single handler. Most of the time a handler targets one aggregate, but it can span multiple aggregates by nesting `repo.Use` calls. Use `command.Aggregate()` to link a command to its primary aggregate:

```go
cmd := command.New(CreateProduct, payload, command.Aggregate(productID, ProductAggregate))
```

**Use `repo.Use` in handlers.** This is the idiomatic fetch-modify-save pattern. `Use` only persists changes when the callback returns `nil`, giving you all-or-nothing semantics:

```go
command.MustHandle(ctx, bus, CreateProduct, func(ctx command.Ctx[CreateProductPayload]) error {
	return products.Use(ctx, ctx.AggregateID(), func(p *Product) error {
		return p.Create(ctx.Payload().Name, ctx.Payload().Price)
	})
})
```

When a command needs to coordinate multiple aggregates, nest the `Use` calls. The outer aggregate acts as the orchestrator:

```go
command.MustHandle(ctx, bus, PlaceOrderCmd, func(ctx command.Ctx[PlaceOrderPayload]) error {
	return orders.Use(ctx, ctx.AggregateID(), func(o *Order) error {
		// Reserve stock on each product first
		for _, item := range ctx.Payload().Items {
			if err := products.Use(ctx, item.ProductID, func(p *Product) error {
				return p.AdjustStock(-item.Quantity, "ordered")
			}); err != nil {
				return err // Stock insufficient — order never placed
			}
		}
		return o.Place(ctx.Payload().CustomerID, ctx.Payload().Items)
	})
})
```

**Prefer synchronous dispatch when you need confirmation.** By default, `Dispatch` is fire-and-forget. Use `dispatch.Sync()` when the caller needs to know the command succeeded:

```go
err := bus.Dispatch(ctx, cmd.Any(), dispatch.Sync())
if err != nil {
	// Command failed — handler returned an error
}
```

## Projection Design

**Choose the right schedule:**
- `Continuously` for real-time read models (dashboards, APIs)
- `Periodically` for batch processing (reports, analytics)

**Use debouncing to batch rapid updates.** If an aggregate raises 10 events in quick succession, debouncing collapses them into a single projection job:

```go
s := schedule.Continuously(bus, store, eventNames, schedule.Debounce(500*time.Millisecond))
```

**Implement `Progressor` for persistent projections.** Without it, the projection re-processes all events from the beginning on every restart:

```go
type ProductCatalog struct {
	*projection.Progressor // Tracks last processed event
	products map[uuid.UUID]ProductView
}
```

**Use `projection.Startup()` for initial catch-up.** On first run, a continuous projection misses all historical events. The `Startup` option triggers a one-time query for past events:

```go
errs, err := s.Subscribe(ctx, applyFn, projection.Startup())
```

**Keep projections idempotent.** Projections may replay events after crashes or restarts. Applying the same event twice should produce the same result.

**Use Guards to filter events.** If a projection only cares about events from a specific aggregate, implement `Guard` to skip irrelevant events before they're processed.

## Error Handling

**Always drain both channels.** `Subscribe` and `Query` return paired channels. If you stop reading from one, the producer goroutine leaks. Use `streams.Walk` — it drains both channels correctly:

```go
events, errs, err := store.Query(ctx, q)
if err != nil {
	return err
}

err = streams.Walk(ctx, func(evt event.Event) error {
	// process event
	return nil
}, events, errs)
```

Or use `streams.Drain` / `streams.All` to collect everything into a slice. See [The Streaming Pattern](/reference/architecture#the-streaming-pattern) for the full set of helpers.

**Cancel contexts to stop streams.** All channel-based APIs respect `context.Context`. Canceling the context closes both channels cleanly.

## Testing

**Unit test aggregates without I/O.** Aggregates are pure state machines — create one, call methods, assert events:

```go
func TestProduct_Create(t *testing.T) {
	p := shop.NewProduct(uuid.New())
	p.Create("Mouse", 2999, 50)

	gtest.Transition(shop.ProductCreated, shop.ProductCreatedData{
		Name: "Mouse", Price: 2999, Stock: 50,
	}).Run(t, p)
}
```

**Integration test with in-memory backends.** No Docker, no infrastructure:

```go
bus := eventbus.New()
store := eventstore.WithBus(eventstore.New(), bus)
repo := repository.New(store)
```

**Test projections by applying events directly:**

```go
catalog := shop.NewProductCatalog()
projection.Apply(catalog, events)
// assert catalog state
```

**Run conformance suites for custom backends.** If you implement a custom event store or bus, the framework provides test suites that verify correctness:

```go
eventstoretest.Run(t, "MyStore", func(enc codec.Encoding) event.Store {
	return mystore.New(enc)
})
```

See [Testing](/guide/testing) for the full testing guide.

## Project Structure

A recommended layout for a goes application:

```
internal/
  shop/
    product.go              # Product aggregate, events, commands, codec registration
    order.go                # Order aggregate, events, commands, codec registration
    customer.go             # Customer aggregate, events, commands, codec registration
    product_catalog.go      # ProductCatalog projection
    order_stats.go          # OrderStats projection
cmd/
  server/
    main.go                 # Wiring: store, bus, repo, handlers, projections
```

**Colocate everything per aggregate.** An aggregate's events, event data types, commands, command payloads, codec registrations, DTOs, and handler setup all belong in the same file. When you open `product.go`, you should see the complete picture — the aggregate struct, every event it can raise, every command it handles, and its `Register` function. No jumping between files to understand what a `Product` can do.

**One file per projection.** Projections tend to grow — they accumulate event handlers, view types, and query methods. Give each projection its own file so it's easy to find and doesn't clutter aggregate files.

Split into separate files only when an aggregate grows large enough to warrant it.
