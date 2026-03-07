# Testing

Event sourcing is highly testable. Aggregates are pure in-memory state machines — no databases, no network calls. You can test them by calling domain methods and asserting the resulting events and state.

> For practical examples, see the [Tutorial](/tutorial/12-testing).

## `gtest` Package

The `gtest` package provides fluent assertions for aggregate testing:

```go
import "github.com/modernice/goes/exp/gtest"
```

Three tools:

- **`gtest.Constructor`** — verify that a constructor correctly initializes the aggregate ID
- **`gtest.Transition`** — assert that an aggregate raised a specific event with matching data
- **`gtest.NonTransition`** — assert that an event was *not* raised

## Testing Constructors

```go
func TestNewProduct(t *testing.T) {
	gtest.Constructor(shop.NewProduct).Run(t)
}
```

This verifies that `NewProduct` properly sets the aggregate ID. Add assertions on the created aggregate with `gtest.Created`:

```go
gtest.Constructor(shop.NewOrder, gtest.Created(func(o *shop.Order) error {
	if o.Placed() {
		return fmt.Errorf("new order should not be placed")
	}
	return nil
})).Run(t)
```

## Testing Transitions

`gtest.Transition` asserts that an aggregate has an uncommitted change with the expected event name and data:

```go
func TestProduct_Create(t *testing.T) {
	p := shop.NewProduct(uuid.New())
	p.Create("Wireless Mouse", 2999, 50)

	gtest.Transition(shop.ProductCreated, shop.ProductCreatedData{
		Name:  "Wireless Mouse",
		Price: 2999,
		Stock: 50,
	}).Run(t, p)
}
```

### Options

| Option | Description |
| --- | --- |
| `gtest.Times(n)` | Expect the event exactly N times |
| `gtest.Once()` | Shorthand for `Times(1)` |

### Variants

| Function | Description |
| --- | --- |
| `Transition(event, data)` | Compare with `==` (data must be `comparable`) |
| `TransitionWithEqual(event, data)` | Compare using `data.Equal()` method |
| `TransitionWithComparer(event, data, fn)` | Compare using a custom function |
| `Signal(event)` | Assert event was raised without checking data |

## Testing Non-Transitions

`gtest.NonTransition` asserts that an event was *not* raised:

```go
func TestOrder_CannotCancelPaidOrder(t *testing.T) {
	o := shop.NewOrder(uuid.New())
	o.Place(customerID, items)
	o.Pay(total)

	o.Cancel("changed mind") // should fail

	gtest.NonTransition(shop.OrderCancelled).Run(t, o)
}
```

## In-Memory Backends

Use the in-memory event store and bus for integration tests — no infrastructure needed:

```go
import (
	"github.com/modernice/goes/aggregate/repository"
	"github.com/modernice/goes/event/eventbus"
	"github.com/modernice/goes/event/eventstore"
)

bus := eventbus.New()
store := eventstore.WithBus(eventstore.New(), bus)
repo := repository.New(store)
products := repository.Typed(repo, shop.NewProduct)
```

Test the full persist-and-fetch cycle:

```go
func TestProduct_PersistAndFetch(t *testing.T) {
	// ... setup store and repo ...

	p := shop.NewProduct(id)
	p.Create("Wireless Mouse", 2999, 50)
	products.Save(ctx, p)

	fetched, _ := products.Fetch(ctx, id)
	if fetched.Name != "Wireless Mouse" {
		t.Errorf("expected name %q, got %q", "Wireless Mouse", fetched.Name)
	}
}
```

## Testing Projections

### Event-by-Event Projections

Apply events directly to the projection:

```go
import "github.com/modernice/goes/projection"

catalog := shop.NewProductCatalog()

events := []event.Event{
	event.New(shop.ProductCreated, shop.ProductCreatedData{
		Name: "Mouse", Price: 2999, Stock: 50,
	}, event.Aggregate(productID, shop.ProductAggregate, 1)).Any(),
}

projection.Apply(catalog, events)

view, ok := catalog.Find(productID)
// assert view...
```

### Full-Stack Projections

For projections that use schedules, wire up the full stack with in-memory backends:

```go
bus := eventbus.New()
store := eventstore.WithBus(eventstore.New(), bus)

// Save aggregates, start projection schedule,
// then assert read model state.
```

See the [Tutorial](/tutorial/12-testing) for complete examples.

## Backend Conformance Suites

If you implement a custom event store or event bus, run the conformance test suites to verify correctness:

```go
import "github.com/modernice/goes/backend/testing/eventstoretest"

func TestMyEventStore(t *testing.T) {
	eventstoretest.Run(t, "MyEventStore", func(enc codec.Encoding) event.Store {
		return mypackage.NewEventStore(enc)
	})
}
```

```go
import "github.com/modernice/goes/backend/testing/eventbustest"

func TestMyEventBus(t *testing.T) {
	eventbustest.RunCore(t, func(enc codec.Encoding) event.Bus {
		return mypackage.NewEventBus(enc)
	})
}
```

The event store suite tests: insert, find, delete, concurrency, and all query operations. The event bus suite tests: basic pub/sub, multiple subscribers, cancellation, and multi-event publishing.
