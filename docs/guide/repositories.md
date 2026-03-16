# Repositories

Repositories are the persistence boundary for aggregates. They save uncommitted events to the event store and reconstruct aggregate state by replaying those events when you fetch an aggregate again.

> For a step-by-step introduction, see the [Tutorial](/tutorial/05-repository).

## Basic Setup

```go
import (
	"github.com/modernice/goes/aggregate/repository"
	"github.com/modernice/goes/event/eventstore"
)

store := eventstore.New()
repo := repository.New(store)
products := repository.Typed(repo, NewProduct)
```

`repository.New(store)` creates an `aggregate.Repository`. `repository.Typed(repo, factory)` wraps it with a type-safe layer that creates and returns your concrete aggregate type.

## Core Methods

| Method | Description |
| --- | --- |
| `Save(ctx, aggregate)` | Insert uncommitted events into the store |
| `Fetch(ctx, id)` | Replay all events to reconstruct current state |
| `FetchVersion(ctx, id, version)` | Reconstruct state up to a specific version |
| `Refresh(ctx, aggregate)` | Reload an already-instantiated aggregate from the latest events |
| `Use(ctx, id, fn)` | Fetch, apply function, save |
| `Query(ctx, query)` | Stream aggregates matching a query |
| `Delete(ctx, aggregate)` | Delete all events for an aggregate |

## The Use Pattern

`Use` is the most common way to modify an aggregate. It fetches the latest state, runs your function, and saves the result:

```go
err := products.Use(ctx, productID, func(p *Product) error {
	return p.ChangePrice(2499)
})
```

If the function returns an error, nothing is saved. This keeps the common fetch-modify-save flow in one place.

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

Aggregates can be soft-deleted without physically removing their events. Implement `aggregate.SoftDeleter` on an event data type:

```go
type ProductDeletedData struct{}

func (ProductDeletedData) SoftDelete() bool { return true }

func (p *Product) Delete() {
	aggregate.Next(p, ProductDeleted, ProductDeletedData{})
}
```

After a soft-delete event has been applied, repository fetches and queries treat the aggregate as deleted. To restore it, implement `aggregate.SoftRestorer` on another event data type:

```go
type ProductRestoredData struct{}

func (ProductRestoredData) SoftRestore() bool { return true }
```

## Cached Repositories

Wrap a typed repository with an in-memory cache to avoid repeated event replays:

```go
cached := repository.Cached(products)

// First call replays events from the store.
p, _ := cached.Fetch(ctx, id)

// Second call returns the cached result.
p, _ = cached.Fetch(ctx, id)

// Invalidate specific entries or the entire cache.
cached.Clear(id)
cached.Clear()
```

The cache stores fully hydrated aggregates. This is useful for read-heavy workloads where the same aggregates are fetched repeatedly.

## Aggregate Queries

Query aggregates by ID or version using `aggregate/query.New`:

```go
import (
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/query"
)

matches, errs, err := products.Query(ctx, query.New(
	query.ID(productID),
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

Typed repositories already know their aggregate name, so `Query` automatically scopes results to the aggregate type created by the factory you passed to `repository.Typed(...)`.

Version constraints use `event/query/version`:

```go
import "github.com/modernice/goes/event/query/version"

// Aggregates at version 5 through 10:
query.Version(version.Min(5), version.Max(10))

// Aggregates at exactly version 1, 2, or 3:
query.Version(version.Exact(1, 2, 3))

// Aggregates in a version range (inclusive):
query.Version(version.InRange(version.Range{10, 20}))
```

## Advanced: Custom Repository Types

If you only need the standard repository behavior, use a simple type alias:

```go
type ProductRepository = aggregate.TypedRepository[*Product]
```

When you need reusable persistence-oriented workflows or repository-local collaborators, prefer composition over reimplementation. Build a custom repository type by embedding `aggregate.TypedRepository[*Product]` and delegate construction to `repository.Typed(...)`.

```go
import (
	"context"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/repository"
)

type ProductRepository struct {
	aggregate.TypedRepository[*Product]

	customField        string
	anotherCustomField map[string]int
}

func NewProductRepository(repo aggregate.Repository) *ProductRepository {
	return &ProductRepository{
		TypedRepository:   repository.Typed(repo, NewProduct),
		customField:       "catalog",
		anotherCustomField: make(map[string]int),
	}
}

func (repo *ProductRepository) ChangePrice(ctx context.Context, id uuid.UUID, price int) error {
	return repo.Use(ctx, id, func(p *Product) error {
		return p.ChangePrice(price)
	})
}
```

`repository.Typed(...)` returns the concrete `*repository.TypedRepository[T]`, but embedding the interface keeps your custom repository's public surface small and flexible.

Use these rules when deciding whether to introduce a custom repository type:

- Use a type alias when no extra behavior is needed.
- Add a custom repository struct only for reusable loading, querying, saving, caching, or secondary lookup workflows.
- Keep domain rules inside the aggregate. Repository methods should orchestrate persistence, not replace aggregate behavior.
- Depend on `aggregate.TypedRepository[*Product]` in most consumers unless they specifically need the added custom methods.

## When To Keep It Simple

Most applications do not need custom repository types. Start with `repository.Typed(...)`, call `Use(...)` from handlers or services, and only add a custom wrapper when the extra methods clearly improve reuse or hide repository-local plumbing.
