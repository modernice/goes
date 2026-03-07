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

`repository.New(store)` creates an untyped repository. `repository.Typed(repo, factory)` wraps it with a type-safe layer that returns concrete aggregate types.

### Methods

| Method | Description |
| --- | --- |
| `Save(ctx, aggregate)` | Insert uncommitted events into the store |
| `Fetch(ctx, id)` | Replay all events to reconstruct current state |
| `FetchVersion(ctx, id, version)` | Reconstruct state up to a specific version |
| `Use(ctx, id, fn)` | Fetch, apply function, save — atomic pattern |
| `Query(ctx, query)` | Stream aggregates matching a query |
| `Delete(ctx, aggregate)` | Delete all events for an aggregate |

### The `Use` Pattern

`Use` is the most common way to modify an aggregate — it fetches the latest state, runs your function, and saves the result:

```go
err := products.Use(ctx, productID, func(p *Product) error {
	return p.ChangePrice(2499)
})
```

If the function returns an error, nothing is saved. This prevents partial updates.

## Repository Options

```go
repo := repository.New(store, opts...)
```

| Option | Description |
| --- | --- |
| `WithSnapshots(store, schedule)` | Use [snapshots](/guide/snapshots) for faster hydration |
| `ValidateConsistency(bool)` | Check event versions before insert (default: `true`) |
| `BeforeInsert(fn)` | Hook called before events are inserted |
| `AfterInsert(fn)` | Hook called after successful insertion |
| `OnFailedInsert(fn)` | Hook called when insertion fails |
| `OnDelete(fn)` | Hook called on aggregate deletion |
| `ModifyQueries(fn...)` | Transform event queries before execution |

## Soft Deletion

Aggregates can be soft-deleted — excluded from queries but not permanently removed. Implement the `SoftDeleter` interface on an event's data type:

```go
type ProductDeletedData struct{}

func (ProductDeletedData) SoftDelete() bool { return true }

func (p *Product) Delete() {
	aggregate.Next(p, ProductDeleted, ProductDeletedData{})
}
```

When a `SoftDeleter` event is applied, the aggregate is excluded from repository queries. To restore it, implement `SoftRestorer`:

```go
type ProductRestoredData struct{}

func (ProductRestoredData) SoftRestore() bool { return true }
```

## Cached Repository

Wrap a typed repository with an in-memory cache to avoid repeated event replays:

```go
cached := repository.Cached(products)

// First call replays events from the store.
p, _ := cached.Fetch(ctx, id)

// Second call returns the cached result.
p, _ = cached.Fetch(ctx, id)

// Invalidate specific entries or the entire cache.
cached.Clear(id)       // clear one
cached.Clear()         // clear all
```

The cache stores fully hydrated aggregates. It's useful for read-heavy workloads where aggregates don't change frequently.

## Aggregate Queries

Query aggregates by name, ID, or version:

```go
import "github.com/modernice/goes/aggregate/query"

products, errs, err := products.Query(ctx, query.New(
	query.Name("shop.product"),
	query.SortBy(aggregate.SortVersion, aggregate.SortDesc),
))
```

### Query Options

| Option | Description |
| --- | --- |
| `Name(names...)` | Filter by aggregate name |
| `ID(ids...)` | Filter by aggregate ID |
| `Version(constraints...)` | Filter by version range |
| `SortBy(field, direction)` | Sort results |

Version constraints use the `version` package — the same package used for [event query version constraints](/guide/events#version-constraints):

```go
import "github.com/modernice/goes/event/query/version"

// Aggregates at version 5 through 10:
query.Version(version.Min(5), version.Max(10))

// Aggregates at exactly version 1, 2, or 3:
query.Version(version.Exact(1, 2, 3))

// Aggregates in a version range (inclusive):
query.Version(version.InRange(version.Range{10, 20}))
```

## Splitting Aggregates

When an aggregate grows large — too many events, too many concerns — you can split it into multiple aggregates that share the same UUID. Each aggregate type gets its own event stream and version history. See the [Aggregate Splitting](/guide/aggregate-splitting) guide for details.
