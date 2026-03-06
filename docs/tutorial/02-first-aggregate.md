# 2. Your First Aggregate

An aggregate is the central building block in event sourcing. It's a consistency boundary — a cluster of domain objects that are treated as a single unit. All state changes go through the aggregate, and all changes are recorded as events.

## The Product Aggregate

Create `product.go`. We'll start with a single event — `ProductCreated`.

```go
package shop

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/event"
)

// Aggregate name — used to identify this aggregate type in the event store.
const ProductAggregate = "shop.product"

// Event name — past tense, describes what happened.
const ProductCreated = "shop.product.created"

// Event data — the payload carried by the event.
type ProductCreatedData struct {
	Name  string
	Price int // in cents
	Stock int
}

// Event type alias — gives a readable name to the typed event.
type ProductCreatedEvent = event.Of[ProductCreatedData]

// ProductDTO holds the read state of a product.
// Including the ID makes the DTO self-contained — when you pass it to
// templates, APIs, or other packages, the ID is always available.
type ProductDTO struct {
	ID    uuid.UUID `json:"id"`
	Name  string    `json:"name"`
	Price int       `json:"price"`
	Stock int       `json:"stock"`
}

// Created reports whether the product has been initialized.
func (dto ProductDTO) Created() bool {
	return dto.Name != ""
}

// Product is our first aggregate.
type Product struct {
	*aggregate.Base
	ProductDTO
}

// NewProduct creates a new Product aggregate with the given ID.
func NewProduct(id uuid.UUID) *Product {
	p := &Product{
		Base: aggregate.New(ProductAggregate, id),
		ProductDTO: ProductDTO{
			ID: id,
		},
	}

	// Register event appliers.
	event.ApplyWith(p, p.created, ProductCreated)

	return p
}

// Create initializes a new product. This is the business method.
func (p *Product) Create(name string, price, stock int) error {
	if p.Created() {
		return fmt.Errorf("product already created")
	}
	if name == "" {
		return fmt.Errorf("product name is required")
	}
	if price <= 0 {
		return fmt.Errorf("price must be positive")
	}
	if stock < 0 {
		return fmt.Errorf("stock cannot be negative")
	}

	// Raise the event. This:
	// 1. Creates the event with the correct aggregate metadata
	// 2. Calls the event applier (p.created)
	// 3. Records the event as an uncommitted change
	aggregate.Next(p, ProductCreated, ProductCreatedData{
		Name:  name,
		Price: price,
		Stock: stock,
	})

	return nil
}

// Event applier — updates the aggregate's state.
// This method is called both when raising new events AND when replaying
// events from the store.
func (p *Product) created(evt ProductCreatedEvent) {
	data := evt.Data()
	p.Name = data.Name
	p.Price = data.Price
	p.Stock = data.Stock
}
```

## Key Concepts

### `aggregate.Base`

Every aggregate embeds `*aggregate.Base`. This provides:
- A UUID identifier
- A name (like `"shop.product"`)
- Version tracking
- Event recording and replay mechanics

```go
p := &Product{
	Base: aggregate.New(ProductAggregate, id),
	ProductDTO: ProductDTO{
		ID: id,
	},
}
```

### The DTO Pattern

The aggregate embeds a DTO struct (`ProductDTO`) that holds the actual state. This keeps your state organized and makes the DTO easy to pass around independently — for example, to serialize as a JSON API response or to use in a template.

Including the `ID` in the DTO means you always have access to the aggregate's identity when working with the DTO directly.

### `event.ApplyWith`

Registers a typed event handler on the aggregate. When an event with the matching name is applied (either from `aggregate.Next` or from replaying stored events), the handler function is called with full type safety.

```go
event.ApplyWith(p, p.created, ProductCreated)
//              ^   ^          ^
//              |   |          event name to match
//              |   handler function (typed)
//              the aggregate (implements event.Registerer)
```

### `aggregate.Next`

Raises a new event on the aggregate. This does three things:

1. Creates an `event.Evt[T]` with the correct aggregate ID, name, and version
2. Calls `ApplyEvent` on the aggregate (which dispatches to your registered handler)
3. Records the event as an uncommitted change via `RecordChange`

```go
aggregate.Next(p, ProductCreated, ProductCreatedData{...})
```

### Business Methods vs. Event Appliers

This is a critical distinction:

- **Business methods** (like `Create`) validate input and raise events. They contain your business rules.
- **Event appliers** (like `created`) update state. They contain no validation — they just apply the data from the event.

Why? Because event appliers are called when *replaying* events from the store. Those events already happened — validation would be nonsensical. Appliers must be pure state transitions.

## Next

The product can only be created so far. In the [next chapter](./03-events-and-state), we'll add more events to manage prices and stock.
