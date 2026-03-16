# Aggregates

Aggregates are consistency boundaries that encapsulate state and enforce business rules. In goes, an aggregate is a Go struct that derives its state from a sequence of events.

> For a step-by-step introduction, see the [Tutorial](/tutorial/02-first-aggregate).

## Defining an Aggregate

An aggregate embeds `*aggregate.Base` and a DTO that holds its state. Event handlers are registered in the constructor using `event.ApplyWith`:

```go
package shop

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/event"
)

const ProductAggregate = "shop.product"

const (
	ProductCreated = "shop.product.created"
	PriceChanged   = "shop.product.price_changed"
)

type ProductDTO struct {
	ID    uuid.UUID `json:"id"`
	Name  string    `json:"name"`
	Price int       `json:"price"`
}

type Product struct {
	*aggregate.Base
	ProductDTO
}

func NewProduct(id uuid.UUID) *Product {
	p := &Product{
		Base:       aggregate.New(ProductAggregate, id),
		ProductDTO: ProductDTO{ID: id},
	}

	event.ApplyWith(p, p.created, ProductCreated)
	event.ApplyWith(p, p.priceChanged, PriceChanged)

	return p
}

func (p *Product) ModelID() uuid.UUID { return p.ProductDTO.ID }
```

`aggregate.New(name, id)` creates a Base with the given aggregate name and UUID. `event.ApplyWith` connects event names to handler methods on the aggregate.

## Raising Events

Domain methods validate the operation, then call `aggregate.Next` to record what happened:

```go
func (p *Product) Create(name string, price int) error {
	if name == "" {
		return fmt.Errorf("name required")
	}
	if price <= 0 {
		return fmt.Errorf("price must be positive")
	}
	aggregate.Next(p, ProductCreated, ProductCreatedData{
		Name:  name,
		Price: price,
	})
	return nil
}

func (p *Product) created(evt event.Of[ProductCreatedData]) {
	p.Name = evt.Data().Name
	p.Price = evt.Data().Price
}
```

`aggregate.Next` does three things in order:

1. Creates an event with the next version number and a monotonically increasing timestamp
2. Applies it to the aggregate immediately (calls the registered handler)
3. Records it as an uncommitted change

Event handlers (appliers) are always unexported. They only mutate state and must not validate or trigger side effects.

## The DTO Pattern

Separating the DTO from the aggregate has practical benefits:

- **Serialization** — the DTO can be marshaled to JSON for APIs or snapshots without exposing `*aggregate.Base`
- **Self-contained** — including the ID in the DTO means it's always available when passed to templates, APIs, or other packages
- **`ModelID()`** — returns the DTO's ID, which integrates with the model repository system for projections

The pattern is: `aggregate.Base` handles event sourcing mechanics, the DTO holds all the business state.

## Repositories

Repositories persist aggregates by saving their events to an event store and reconstructing state by replaying them:

```go
import (
	"github.com/modernice/goes/aggregate/repository"
	"github.com/modernice/goes/event/eventstore"
)

store := eventstore.New()
repo := repository.New(store)
products := repository.Typed(repo, NewProduct)
```

`repository.New(store)` creates an `aggregate.Repository`. `repository.Typed(repo, factory)` wraps it with a type-safe layer that returns concrete aggregate types.

For repository methods, options, queries, soft deletion, caching, and advanced custom repository design, see the [Repositories](/guide/repositories) guide.

## Splitting Aggregates

When an aggregate grows large — too many events, too many concerns — you can split it into multiple aggregates that share the same UUID. Each aggregate type gets its own event stream and version history. See the [Aggregate Splitting](/guide/aggregate-splitting) guide for details.
