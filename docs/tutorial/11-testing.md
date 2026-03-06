# 11. Testing

One of the best things about event sourcing is how testable it is. Aggregates are pure in-memory state machines — no databases, no network calls. You can test them by raising events and asserting state.

## `gtest`

goes provides the `gtest` package for testing aggregates. It lets you assert that domain methods raise the correct events with the expected data:

```go
import "github.com/modernice/goes/exp/gtest"
```

The main tools are:

- **`gtest.Constructor(fn)`** — verify that a constructor correctly initializes the aggregate ID.
- **`gtest.Transition(eventName, data)`** — assert that an aggregate raised a specific event with matching data.
- **`gtest.NonTransition(eventName)`** — assert that an event was *not* raised.

## Testing Constructors

`gtest.Constructor` verifies that your constructor properly sets the aggregate ID. Use the `gtest.Created` option to run additional assertions on the created aggregate:

```go
package shop_test

import (
	"fmt"
	"testing"

	"example.com/shop"
	"github.com/modernice/goes/exp/gtest"
)

func TestNewProduct(t *testing.T) {
	gtest.Constructor(shop.NewProduct).Run(t)
}

func TestNewOrder(t *testing.T) {
	gtest.Constructor(shop.NewOrder, gtest.Created(func(o *shop.Order) error {
		if o.Placed() {
			return fmt.Errorf("new order should not be placed")
		}
		return nil
	})).Run(t)
}
```

## Testing Aggregates

Create, execute business methods, and assert the resulting events:

```go
package shop_test

import (
	"testing"

	"example.com/shop"
	"github.com/google/uuid"
	"github.com/modernice/goes/exp/gtest"
)

func TestProduct_Create(t *testing.T) {
	p := shop.NewProduct(uuid.New())
	if err := p.Create("Wireless Mouse", 2999, 50); err != nil {
		t.Fatal(err)
	}

	gtest.Transition(shop.ProductCreated, shop.ProductCreatedData{
		Name:  "Wireless Mouse",
		Price: 2999,
		Stock: 50,
	}).Run(t, p)
}

func TestProduct_Create_Validation(t *testing.T) {
	p := shop.NewProduct(uuid.New())

	if err := p.Create("", 2999, 50); err == nil {
		t.Error("expected error for empty name")
	}

	if err := p.Create("Mouse", 0, 50); err == nil {
		t.Error("expected error for zero price")
	}

	if err := p.Create("Mouse", 2999, -1); err == nil {
		t.Error("expected error for negative stock")
	}
}
```

## Testing State After Multiple Events

```go
func TestProduct_AdjustStock(t *testing.T) {
	p := shop.NewProduct(uuid.New())
	p.Create("Wireless Mouse", 2999, 50)

	if err := p.AdjustStock(-10, "sold"); err != nil {
		t.Fatal(err)
	}

	gtest.Transition(shop.StockAdjusted, shop.StockAdjustedData{
		Quantity: -10,
		Reason:   "sold",
	}).Run(t, p)

	if p.Stock != 40 {
		t.Errorf("expected stock %d, got %d", 40, p.Stock)
	}

	// Cannot go below zero.
	if err := p.AdjustStock(-50, "oversell"); err == nil {
		t.Error("expected error for insufficient stock")
	}
}
```

## Testing the Order Lifecycle

```go
func TestOrder_Lifecycle(t *testing.T) {
	o := shop.NewOrder(uuid.New())

	// Place an order.
	items := []shop.LineItem{
		{ProductID: uuid.New(), Name: "Mouse", Price: 2999, Quantity: 2},
	}
	if err := o.Place(uuid.New(), items); err != nil {
		t.Fatal(err)
	}
	if o.Total != 5998 {
		t.Errorf("expected total %d, got %d", 5998, o.Total)
	}

	// Pay the order.
	if err := o.Pay(5998); err != nil {
		t.Fatal(err)
	}

	gtest.Transition(shop.OrderPaid, 5998).Run(t, o)

	// Cannot cancel a paid order.
	if err := o.Cancel("changed mind"); err == nil {
		t.Error("expected error cancelling paid order")
	}

	gtest.NonTransition(shop.OrderCancelled).Run(t, o)
}
```

## Testing With Repositories

To test the full persist-and-fetch cycle, use the in-memory event store:

```go
import (
	"context"
	"testing"

	"example.com/shop"
	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate/repository"
	"github.com/modernice/goes/event/eventstore"
)

func TestProduct_PersistAndFetch(t *testing.T) {
	ctx := context.Background()

	store := eventstore.New()
	repo := repository.New(store)
	products := repository.Typed(repo, shop.NewProduct)

	// Create and save.
	id := uuid.New()
	p := shop.NewProduct(id)
	p.Create("Wireless Mouse", 2999, 50)
	p.AdjustStock(-5, "sold")

	if err := products.Save(ctx, p); err != nil {
		t.Fatal(err)
	}

	// Fetch and verify state was reconstructed from events.
	fetched, err := products.Fetch(ctx, id)
	if err != nil {
		t.Fatal(err)
	}

	if fetched.ProductDTO.Name != "Wireless Mouse" {
		t.Errorf("expected name %q, got %q", "Wireless Mouse", fetched.ProductDTO.Name)
	}
	if fetched.Stock != 45 {
		t.Errorf("expected stock %d, got %d", 45, fetched.Stock)
	}
}
```

## Testing Projections

> [!NOTE]
> goes doesn't yet provide dedicated testing tools for projections like it does for aggregates with `gtest`. For now, you test projections by wiring them up manually. Simpler projections can be tested by applying events directly; more complex ones require the full event store and repository stack.

Test simple event-by-event projections by applying events manually:

```go
import (
	"example.com/shop"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/projection"
)

func TestProductCatalog(t *testing.T) {
	catalog := shop.NewProductCatalog()

	productID := uuid.New()

	// Apply events directly to the projection.
	events := []event.Event{
		event.New(shop.ProductCreated, shop.ProductCreatedData{
			Name:  "Wireless Mouse",
			Price: 2999,
			Stock: 50,
		}, event.Aggregate(productID, shop.ProductAggregate, 1)).Any(),

		event.New(shop.PriceChanged, 2499, event.Aggregate(productID, shop.ProductAggregate, 2)).Any(),
	}

	projection.Apply(catalog, events)

	view, ok := catalog.Find(productID)
	if !ok {
		t.Fatal("product not found in catalog")
	}
	if view.Price != 2499 {
		t.Errorf("expected price %d, got %d", 2499, view.Price)
	}
}
```

## Testing Rebuild-Style Projections

For projections that rebuild from aggregates, you need the full event store and repository stack. Save aggregates, run the projector, then assert the read model:

```go
import (
	"example.com/shop"
	"github.com/modernice/goes/backend/memory"
	"github.com/modernice/goes/event/eventbus"
	"github.com/modernice/goes/helper/streams"
)

func TestOrderSummaryProjector(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus := eventbus.New()
	store := eventstore.WithBus(eventstore.New(), bus)

	repo := repository.New(store)
	customers := repository.Typed(repo, shop.NewCustomer)
	orders := repository.Typed(repo, shop.NewOrder)

	// Save a customer.
	customerID := uuid.New()
	c := shop.NewCustomer(customerID)
	c.Register("Jane Doe", "jane@example.com")
	if err := customers.Save(ctx, c); err != nil {
		t.Fatal(err)
	}

	// Save an order.
	orderID := uuid.New()
	o := shop.NewOrder(orderID)
	o.Place(customerID, []shop.LineItem{
		{ProductID: uuid.New(), Name: "Mouse", Price: 2999, Quantity: 1},
	})
	o.Pay(2999)
	if err := orders.Save(ctx, o); err != nil {
		t.Fatal(err)
	}

	// Set up the projector.
	summaries := memory.NewModelRepository[*shop.OrderSummary, uuid.UUID](
		memory.ModelFactory(shop.OrderSummaryOf),
	)
	projector := shop.NewOrderSummaryProjector(customers, orders, summaries)

	errs, err := projector.Run(ctx, bus, store)
	if err != nil {
		t.Fatal(err)
	}
	go streams.Drain(errs)

	// Wait briefly for the startup projection to process.
	time.Sleep(100 * time.Millisecond)

	// Verify the summary.
	summary, err := summaries.Fetch(ctx, orderID)
	if err != nil {
		t.Fatal(err)
	}
	if summary.CustomerName != "Jane Doe" {
		t.Errorf("expected customer name %q, got %q", "Jane Doe", summary.CustomerName)
	}
	if summary.Status != shop.OrderStatusPaid {
		t.Errorf("expected status %v, got %v", shop.OrderStatusPaid, summary.Status)
	}
	if summary.Total != 2999 {
		t.Errorf("expected total %d, got %d", 2999, summary.Total)
	}
}
```

## Testing Command Handlers

For integration tests with command handling, wire up the full stack with in-memory backends:

```go
func TestCreateProductCommand(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reg := codec.New()
	shop.RegisterProductEvents(reg)
	shop.RegisterProductCommands(reg)

	bus := eventbus.New()
	store := eventstore.WithBus(eventstore.New(), bus)

	repo := repository.New(store)
	products := repository.Typed(repo, shop.NewProduct)
	cbus := cmdbus.New[int](reg, bus)

	errs := shop.HandleProductCommands(ctx, cbus, products)
	go streams.Drain(errs)

	// Dispatch a command.
	id := uuid.New()
	cmd := command.New(shop.CreateProductCmd, shop.CreateProductPayload{
		Name:  "Wireless Mouse",
		Price: 2999,
		Stock: 50,
	}, command.Aggregate(shop.ProductAggregate, id))

	if err := cbus.Dispatch(ctx, cmd.Any()); err != nil {
		t.Fatal(err)
	}

	// Verify the aggregate was created.
	p, err := products.Fetch(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if p.ProductDTO.Name != "Wireless Mouse" {
		t.Errorf("expected name %q, got %q", "Wireless Mouse", p.ProductDTO.Name)
	}
}
```

## Testing Tips

1. **Test aggregates in isolation first** — no store, no bus, just create and call methods. This is the fastest and most valuable layer of tests.

2. **Use `gtest` for event assertions** — `gtest.Transition` and `gtest.NonTransition` verify that the correct events were raised (or not raised) without manually inspecting `AggregateChanges()`.

3. **Use in-memory backends** — `eventstore.New()` and `eventbus.New()` require no infrastructure. Use them for integration tests.

4. **Test validation boundaries** — ensure your aggregate rejects invalid operations (empty names, negative prices, wrong status transitions).

## What You've Built

Congratulations! You've built a complete event-sourced e-commerce application with:

- **3 aggregates** — Product, Order, Customer
- **Type-safe events and commands** — with codec registration
- **Typed repositories** — persist and fetch aggregates
- **A product catalog projection** — reactive read model
- **Production backends** — MongoDB event store, NATS event bus
- **Tests** — for aggregates, projections, and command handlers

## What's Next?

- **[Guide](/guide/aggregates)** — deep-dive into each framework component
- **[Backends](/backends/)** — detailed backend configuration and options
- **[Reference](/reference/architecture)** — architecture overview and best practices
