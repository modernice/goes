# Events

Events are immutable records of things that happened in your system. They are the foundation of event sourcing — all state is derived from replaying events.

> For a step-by-step introduction, see the [Tutorial](/tutorial/03-events-and-state).

## Defining Events

An event has a name and a data type. Define event names as constants and data types as structs:

```go
const (
	ProductCreated = "shop.product.created"
	PriceChanged   = "shop.product.price_changed"
)

type ProductCreatedData struct {
	Name  string `json:"name"`
	Price int    `json:"price"`
	Stock int    `json:"stock"`
}
```

The `event.Of[Data]` interface represents a typed event:

```go
type ProductCreatedEvent = event.Of[ProductCreatedData]
```

`event.Event` is an alias for `event.Of[any]` — an event with untyped data.

## Creating Events

```go
evt := event.New(ProductCreated, ProductCreatedData{
	Name:  "Wireless Mouse",
	Price: 2999,
	Stock: 50,
})
```

`event.New` assigns a random UUID and the current time automatically.

### Options

| Option | Description |
| --- | --- |
| `event.ID(uuid)` | Override the event ID |
| `event.Time(t)` | Override the timestamp |
| `event.Aggregate(id, name, version)` | Link the event to an aggregate |
| `event.Previous(evt)` | Copy aggregate info from another event, incrementing the version |

In practice, you rarely call `event.New` directly — aggregates use `aggregate.Next` which handles event creation, versioning, and application automatically.

## Registering Event Handlers

`event.ApplyWith` connects event names to handler methods on an aggregate or projection:

```go
func NewProduct(id uuid.UUID) *Product {
	p := &Product{Base: aggregate.New(ProductAggregate, id)}

	event.ApplyWith(p, p.created, ProductCreated)
	event.ApplyWith(p, p.priceChanged, PriceChanged)

	return p
}

func (p *Product) created(evt event.Of[ProductCreatedData]) {
	p.Name = evt.Data().Name
	p.Price = evt.Data().Price
}
```

The handler receives a typed event — `event.Of[ProductCreatedData]` — so you access data with `evt.Data()` without type assertions.

A single handler can be registered for multiple event names:

```go
event.ApplyWith(p, p.handleStockChange, StockAdjusted, StockReset)
```

## Type Casting

When you receive an `event.Event` (untyped) and need the typed data:

```go
// Cast — panics if the type doesn't match:
typed := event.Cast[ProductCreatedData](evt)
data := typed.Data() // ProductCreatedData

// TryCast — returns false if the type doesn't match:
typed, ok := event.TryCast[ProductCreatedData](evt)

// Any — convert a typed event to untyped:
untyped := event.Any(typedEvt) // event.Event
```

## Event Store

The event store persists events. All implementations satisfy the `event.Store` interface:

```go
type Store interface {
	Insert(ctx context.Context, events ...Event) error
	Find(ctx context.Context, id uuid.UUID) (Event, error)
	Query(ctx context.Context, q Query) (<-chan Event, <-chan error, error)
	Delete(ctx context.Context, events ...Event) error
}
```

```go
// In-memory (for dev/testing):
import "github.com/modernice/goes/event/eventstore"
store := eventstore.New()

// MongoDB or PostgreSQL for production — see Backends.
```

`Query` returns channels, suitable for streaming large result sets.

## Event Bus

The event bus distributes events to subscribers. All implementations satisfy the `event.Bus` interface:

```go
type Bus interface {
	Publish(ctx context.Context, events ...Event) error
	Subscribe(ctx context.Context, names ...string) (<-chan Event, <-chan error, error)
}
```

```go
// In-memory (for dev/testing):
import "github.com/modernice/goes/event/eventbus"
bus := eventbus.New()

// NATS for production — see Backends.
```

`Subscribe` returns an event channel and an error channel. The channels close when the context is canceled.

## Auto-Publishing with `WithBus`

The `eventstore.WithBus` decorator automatically publishes events to the bus whenever they're inserted into the store:

```go
store := eventstore.WithBus(eventstore.New(), bus)

// Now any Insert() also publishes to the bus.
```

This is the standard wiring — it ensures the bus always stays in sync with the store without manual publish calls. The repository's `Save` method calls `Insert`, so saved aggregate events are published automatically.

## Event Queries

Build queries with the `query` package:

```go
import "github.com/modernice/goes/event/query"

events, errs, err := store.Query(ctx, query.New(
	query.Name("shop.product.created", "shop.product.price_changed"),
	query.AggregateName("shop.product"),
	query.AggregateID(productID),
	query.SortByTime(),
))
```

### Query Options

| Option | Description |
| --- | --- |
| `Name(names...)` | Filter by event name |
| `ID(ids...)` | Filter by event ID |
| `Time(constraints...)` | Filter by timestamp |
| `AggregateName(names...)` | Filter by aggregate name |
| `AggregateID(ids...)` | Filter by aggregate ID |
| `AggregateVersion(constraints...)` | Filter by aggregate version |
| `Aggregate(name, id)` | Shorthand for aggregate name + ID |
| `SortBy(field, direction)` | Sort results |
| `SortByTime()` | Sort by time ascending |
| `SortByAggregate()` | Sort by aggregate name, ID, then version |

### Version Constraints

Version constraints filter events by their aggregate version. Import the `version` package:

```go
import "github.com/modernice/goes/event/query/version"
```

| Constraint | Description |
| --- | --- |
| `version.Exact(v...)` | Match specific versions |
| `version.Min(v...)` | Version must be ≥ at least one value |
| `version.Max(v...)` | Version must be ≤ at least one value |
| `version.InRange(ranges...)` | Version must fall within at least one range |

Combine constraints in a single `AggregateVersion` call — all constraint types must be satisfied:

```go
// Events from versions 5 through 10:
query.AggregateVersion(version.Min(5), version.Max(10))

// Events at exactly version 1, 2, or 3:
query.AggregateVersion(version.Exact(1, 2, 3))

// Events in a version range (inclusive):
query.AggregateVersion(version.InRange(version.Range{10, 20}))
```

`version.Range` is a `[2]int` — the first element is the start, the second is the end (both inclusive). It provides `Start()`, `End()`, and `Includes(v)` methods.

### Time Constraints

Time constraints filter events by their timestamp. The package name conflicts with the standard library, so alias it on import:

```go
import gotime "github.com/modernice/goes/event/query/time"
```

| Constraint | Description |
| --- | --- |
| `gotime.After(t)` | Timestamp must be strictly after `t` |
| `gotime.Before(t)` | Timestamp must be strictly before `t` |
| `gotime.Min(t)` | Timestamp must be ≥ `t` |
| `gotime.Max(t)` | Timestamp must be ≤ `t` |
| `gotime.Exact(t...)` | Match specific timestamps |
| `gotime.InRange(ranges...)` | Timestamp must fall within at least one range |

`After` and `Before` are convenience wrappers around `Min` and `Max` that shift by one nanosecond to provide exclusive boundaries.

```go
// Events from the last 24 hours:
query.Time(gotime.After(time.Now().Add(-24 * time.Hour)))

// Events between two points in time (inclusive):
query.Time(gotime.Min(startTime), gotime.Max(endTime))

// Events in a time range:
query.Time(gotime.InRange(gotime.Range{startTime, endTime}))
```

`gotime.Range` is a `[2]time.Time` — the first element is the start, the second is the end. Like `version.Range`, it provides `Start()`, `End()`, and `Includes(t)` methods.

## Sorting

Events can be sorted by:

| Constant | Description |
| --- | --- |
| `event.SortTime` | Event timestamp |
| `event.SortAggregateName` | Aggregate name |
| `event.SortAggregateID` | Aggregate UUID |
| `event.SortAggregateVersion` | Aggregate version |

With direction `event.SortAsc` or `event.SortDesc`:

```go
query.SortBy(event.SortTime, event.SortAsc)
```
