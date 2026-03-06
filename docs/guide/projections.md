# Projections

Projections build read-optimized views from events. While aggregates enforce consistency on the write side, projections serve the read side — tailored for specific query needs and updated reactively as events flow through the system.

> For a step-by-step introduction, see the [Tutorial](/tutorial/09-projections).

## `projection.Base`

`projection.Base` provides event handler registration for projections, the same way `aggregate.Base` does for aggregates:

```go
import (
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/projection"
)

type ProductCatalog struct {
	*projection.Base
	products map[uuid.UUID]ProductView
}

func NewProductCatalog() *ProductCatalog {
	c := &ProductCatalog{
		Base:     projection.New(),
		products: make(map[uuid.UUID]ProductView),
	}

	event.ApplyWith(c, c.productCreated, ProductCreated)
	event.ApplyWith(c, c.priceChanged, PriceChanged)

	return c
}
```

`projection.Base` provides:

| Method | Description |
| --- | --- |
| `RegisterEventHandler(name, fn)` | Register a handler for an event name |
| `ApplyEvent(evt)` | Apply an event by calling the registered handler |
| `RegisteredEvents()` | Return the list of registered event names |

You typically use `event.ApplyWith` rather than calling these directly.

## `projection.Progressor`

The Progressor tracks which events have already been applied. It records the timestamp and IDs of the last processed events, so the schedule can skip events that were already handled.

```go
type ShopStats struct {
	*projection.Base
	*projection.Progressor
	// ...
}

func NewShopStats(id uuid.UUID) *ShopStats {
	s := &ShopStats{
		Base:       projection.New(),
		Progressor: projection.NewProgressor(),
	}
	// ...
	return s
}
```

The Progressor only makes sense for **persisted projections**. In-memory projections rebuild from scratch on every restart, so tracking progress has no benefit.

| Method | Description |
| --- | --- |
| `Progress()` | Returns `(time.Time, []uuid.UUID)` — last event time and IDs |
| `SetProgress(t, ids...)` | Sets the progress marker |

The schedule's `Apply` method updates progress automatically when the projection implements `ProgressAware`.

## Schedules

Schedules control when and how projection jobs run. Three strategies are available:

### Continuous

```go
import "github.com/modernice/goes/projection/schedule"

s := schedule.Continuously(bus, store, []string{
	ProductCreated, PriceChanged,
})
```

Triggers a job immediately when a matching event arrives on the bus. This is the most common choice — projections stay up to date in real time.

### Periodic

```go
s := schedule.Periodically(store, 30*time.Second, []string{
	ProductCreated, OrderPlaced, OrderPaid,
})
```

Triggers a job at fixed intervals by querying the event store. No event bus needed. Useful for background projections like dashboards or reports that don't need real-time updates.

### Debouncing

```go
s := schedule.Continuously(bus, store, eventNames,
	schedule.Debounce(time.Second),
)
```

Batches multiple events within a time window into a single job. If 10 events arrive within 1 second, the projection runs once instead of 10 times. The `DebounceCap` option sets the maximum wait time before force-triggering.

## Subscribing

Subscribe to a schedule to start processing jobs:

```go
errs, err := s.Subscribe(ctx, func(job projection.Job) error {
	return job.Apply(job, catalog)
}, projection.Startup())
```

The `projection.Startup()` option triggers an initial catch-up run that processes all existing events from the store. Without it, only new events (arriving after subscription) are processed.

The returned error channel reports asynchronous errors during job processing.

## Jobs

A `projection.Job` represents a batch of events to process. It implements `context.Context` and provides:

| Method | Description |
| --- | --- |
| `Apply(ctx, target, opts...)` | Apply events to a projection target |
| `Events(ctx, filters...)` | Stream all job events, optionally filtered |
| `EventsFor(ctx, target)` | Stream events respecting the target's progress |
| `Aggregates(ctx, names...)` | Stream deduplicated aggregate references |
| `Aggregate(ctx, name)` | Get the UUID of the first aggregate with the given name |

### Apply Options

| Option | Description |
| --- | --- |
| `IgnoreProgress()` | Apply all events regardless of the target's progress |

`Apply` applies events one-by-one to update the projection incrementally. This works well when each event maps directly to a state change in the read model.

Sometimes you need the full current state of an aggregate rather than individual event deltas — for example, when syncing a denormalized view that stores a snapshot of each product. In that case, use `Aggregates` to discover which aggregates were affected by the job's events, fetch them from the repository, and write their current state to the read model:

```go
// catalog is the ProductCatalog projection defined above.
// repo is a typed aggregate repository for Product.
errs, err := s.Subscribe(ctx, func(job projection.Job) error {
	refs, errs, err := job.Aggregates(job)
	if err != nil {
		return err
	}

	for ref := range refs {
		p, err := repo.Fetch(job, ref.ID)
		if err != nil {
			return err
		}
		catalog.products[p.ModelID()] = ProductView{
			Name:  p.Name,
			Price: p.Price,
		}
	}

	return streams.Walk(job, func(_ error) error { return nil }, errs)
})
```

## Guards

Guards filter which events reach a projection during `job.Apply`. When `Apply` iterates over the job's events, it calls `GuardProjection` for each one and skips those that return `false`. This only matters for event-by-event projections — if you write your own subscription handler using `job.Aggregates`, you control the logic yourself and guards are never consulted.

Implement the `Guard` interface:

```go
type Guard interface {
	GuardProjection(event.Event) bool
}
```

If `GuardProjection` returns `false`, the event is skipped.

### QueryGuard

`QueryGuard` is a type alias for `query.Query` that implements the `Guard` interface — only events matching the query pass through:

```go
type FilteredProjection struct {
	*projection.Base
	projection.QueryGuard
}

func NewFilteredProjection() *FilteredProjection {
	p := &FilteredProjection{
		Base:       projection.New(),
		QueryGuard: projection.QueryGuard(query.New(
			query.AggregateName("shop.product"),
		)),
	}
	// ...
	return p
}
```

### Custom Guards

For more complex filtering, use the `guard` package:

```go
import "github.com/modernice/goes/projection/guard"

g := guard.New(
	guard.Event("shop.order.placed", func(evt event.Of[OrderPlacedData]) bool {
		return evt.Data().Total > 10000 // only large orders
	}),
)
```

## Projection Service

Schedules run projections automatically — on every event or at intervals. But sometimes you need to trigger a projection manually: an admin wants to force a rebuild, or a migration script needs to re-project after a schema change. The projection service gives you named, on-demand triggering over the event bus.

Because it communicates through the event bus, triggering works across process boundaries. An admin API in one service can trigger a projection running in another.

### Setup

Create schedules as usual, then register them with the service under a name:

```go
// Create the schedules.
catalogSchedule := schedule.Continuously(bus, store, []string{
	ProductCreated, PriceChanged,
})
statsSchedule := schedule.Periodically(store, 30*time.Second, []string{
	OrderPlaced, OrderPaid,
})

// Create the service with the event bus and register the schedules.
svc := projection.NewService(bus)
svc.Register("product-catalog", catalogSchedule)
svc.Register("shop-stats", statsSchedule)

// Start listening for trigger events.
errs, err := svc.Run(ctx)
```

`Run` subscribes to trigger events on the bus in a background goroutine. When a trigger arrives for a registered schedule name, the service executes that schedule. The returned error channel reports asynchronous errors from handling triggers.

### Triggering

Trigger a projection by name from anywhere that has access to the same event bus:

```go
err := svc.Trigger(ctx, "product-catalog")
```

`Trigger` publishes a trigger event on the bus and waits for acknowledgment from the service that handles that schedule. If no service picks it up within the timeout (default 5s), it returns `ErrUnhandledTrigger`.

You can pass options to control how the projection runs:

```go
err := svc.Trigger(ctx, "product-catalog", projection.Reset(true))
```

## Resetting

Projections can be reset — clearing their progress and rebuilding from scratch. Implement the `Resetter` interface:

```go
type Resetter interface {
	Reset()
}

func (c *ProductCatalog) Reset() {
	c.products = make(map[uuid.UUID]ProductView)
}
```

Trigger a reset through the schedule:

```go
s.Trigger(ctx, projection.Reset(true))
```

This clears the projection's progress, calls `Reset()` if implemented, and re-applies all events from the store.

## BeforeEvent Interceptors

Insert synthetic events before a matched event during job processing:

```go
errs, err := s.Subscribe(ctx, applyFn,
	projection.BeforeEvent(func(ctx context.Context, evt event.Event) ([]event.Event, error) {
		// Return events to insert before OrderPlaced.
		return []event.Event{
			event.New("order.enrichment", EnrichmentData{
				Source: "inventory-check",
			}).Any(),
		}, nil
	}, OrderPlaced),
)
```

`BeforeEvent` is generic — the type parameter controls how the event is cast before it reaches your function. Using `event.Event` (as above) gives you the raw event. If you need typed data, use the specific data type (e.g., `event.Of[OrderPlacedData]`) and the cast happens automatically.

This is useful for enrichment — injecting additional context before the main event is processed.
