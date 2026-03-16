# 5. Repositories

We can create products and raise events, but those events only exist in memory on the aggregate. To persist them — and later reconstruct the aggregate by replaying its events — we need a repository.

## Set Up the Repository

In `cmd/main.go`, create a typed repository for products:

```go
import (
	// ... existing imports ...
	"github.com/modernice/goes/aggregate/repository"
	"github.com/yourname/shop"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	eventReg := codec.New()
	shop.RegisterProductEvents(eventReg)

	store := eventstore.New()
	bus := eventbus.New()
	store = eventstore.WithBus(store, bus)

	// Create the base aggregate repository.                  // [!code ++]
	repo := repository.New(store)                             // [!code ++]
	                                                          // [!code ++]
	// Create a typed repository for products.                // [!code ++]
	products := repository.Typed(repo, shop.NewProduct)       // [!code ++]

	_ = products
	// ...
}
```

## Save an Aggregate

```go
id := uuid.New()
product := shop.NewProduct(id)
product.Create("Wireless Mouse", 2999, 50) // raises ProductCreated event

// Save persists the uncommitted events to the event store.
if err := products.Save(ctx, product); err != nil {
	log.Fatal(err)
}
```

After `Save`, the events are written to the event store and the aggregate's changes are cleared.

## Fetch an Aggregate

```go
// Fetch reads the aggregate's events from the store and replays them.
fetched, err := products.Fetch(ctx, id)
if err != nil {
	log.Fatal(err)
}

fmt.Println(fetched.Name)  // "Wireless Mouse"
fmt.Println(fetched.Price) // 2999
fmt.Println(fetched.Stock) // 50
```

`Fetch` does the following:

1. Creates a new `Product` using the `NewProduct` function you passed to `repository.Typed`
2. Queries the event store for all events belonging to this aggregate
3. Replays them in order, calling the registered event appliers
4. Returns the fully reconstructed aggregate

## The `Use` Pattern

The most common pattern is fetch → modify → save. The `Use` method wraps this in a single call:

```go
err := products.Use(ctx, id, func(p *shop.Product) error {
	return p.Rename("Ergonomic Wireless Mouse")
})
```

`Use` does:

1. **Fetch** — reconstruct the aggregate from its events
2. **Call your function** — modify the aggregate
3. **Save** — if your function returns `nil`, persist the new events

If your function returns an error, the aggregate is not saved.

## How Repositories Work

```
┌─────────────────────────────────┐
│  products.Save(ctx, product)    │
│                                 │
│  1. Get uncommitted changes     │
│  2. Insert events into store    │
│  3. Clear changes on aggregate  │
└─────────────────────────────────┘

┌─────────────────────────────────┐
│  products.Fetch(ctx, id)        │
│                                 │
│  1. NewProduct(id)              │
│  2. Query events from store     │
│  3. Replay events (ApplyEvent)  │
│  4. Return aggregate            │
└─────────────────────────────────┘
```

## Why `repository.Typed`?

`repository.New` returns a base repository that works with the generic `aggregate.Aggregate` interface. You have to create the aggregate yourself and pass it in:

```go
repo := repository.New(store)

// Save — pass the aggregate in.
repo.Save(ctx, product)

// Fetch — create the aggregate yourself, then pass it in to be populated.
p := shop.NewProduct(id)
repo.Fetch(ctx, p)

// p is now populated with replayed events.
fmt.Println(p.Name)
```

`repository.Typed` wraps this with type safety. It uses the `NewProduct` function you pass in to create aggregates internally, and returns the concrete type:

```go
products := repository.Typed(repo, shop.NewProduct)

// Save — accepts *Product.
products.Save(ctx, product)

// Fetch — creates the aggregate for you and returns *Product.
p, err := products.Fetch(ctx, id)

fmt.Println(p.Name)
```

Throughout this tutorial, we'll always use the typed repository.

## Next

We have persistence. In the [next chapter](./06-commands), we'll add a command system to decouple intent from execution.
