# Architecture

goes is a layered event-sourcing framework. Each layer depends only on the layers below it, and all communication between layers happens through Go interfaces — making every component swappable.

> For hands-on examples of each layer, see the [Tutorial](/tutorial/) and the [Guide](/guide/aggregates).

## Package Overview

```
                    codec (cross-cutting)
                      │
              ┌───────┴───────┐
              │               │
            event ◄────── command
              │               │
              └───────┬───────┘
                      │
                  aggregate
                      │
                  repository
                      │
              ┌───────┴───────┐
              │               │
          projection       backend
```

| Package | Purpose |
| --- | --- |
| `event` | Core event types, event store and event bus interfaces, query builder |
| `command` | Command types, command bus interface, handler registration |
| `aggregate` | Event-sourced aggregate base type, event handler registration |
| `aggregate/repository` | Persistence facade — save, fetch, and query aggregates |
| `projection` | Read-model [projections](/guide/projections) with scheduling and progress tracking |
| `codec` | Maps event/command names to Go types in the [codec registry](/guide/codec) |
| `backend` | Production implementations ([MongoDB](/backends/mongodb), [PostgreSQL](/backends/postgres), [NATS](/backends/nats)) |

## The Write Path

The write path flows from a command dispatch to persisted events:

```
Dispatch ──► Command Bus ──► Handler ──► repo.Use() ──► Store.Insert ──► Bus.Publish
                                            │
                                     Fetch ──► Method ──► Save
```

### Step by step

**1. Dispatch a command.** A command carries a name, a typed payload, and optionally an aggregate target:

```go
cmd := command.New(CreateProduct, CreateProductPayload{
	Name:  "Wireless Mouse",
	Price: 2999,
}, command.Aggregate(productID, ProductAggregate))

bus.Dispatch(ctx, cmd.Any(), dispatch.Sync())
```

**2. The command bus routes to a handler.** The bus uses the event bus internally for distributed coordination. It publishes a series of coordination events (`CommandDispatched` → `CommandRequested` → `CommandAssigned` → `CommandAccepted` → `CommandExecuted`) to ensure exactly one handler processes the command, even across multiple services.

**3. The handler loads the aggregate and calls a domain method.** The typical pattern uses `repo.Use`, which fetches the aggregate, runs the callback, and saves — all in one call:

```go
command.MustHandle(ctx, bus, CreateProduct, func(ctx command.Ctx[CreateProductPayload]) error {
	return products.Use(ctx, ctx.AggregateID(), func(p *Product) error {
		return p.Create(ctx.Payload().Name, ctx.Payload().Price)
	})
})
```

**4. The domain method raises events.** Inside `aggregate.Next`, each event is applied immediately to the aggregate's state and recorded as an uncommitted change. Events get auto-incremented versions and monotonic timestamps:

```go
func (p *Product) Create(name string, price int) error {
	aggregate.Next(p, ProductCreated, ProductCreatedData{
		Name:  name,
		Price: price,
	})
	return nil
}
```

**5. The repository inserts events into the store.** When `repo.Use` calls `Save`, it extracts the aggregate's uncommitted changes and inserts them into the event store.

**6. The store publishes events to the bus.** If the store is wrapped with `eventstore.WithBus(store, bus)`, events are automatically published to the event bus after insertion. This is what triggers the read path.

## The Read Path

The read path flows from published events to updated projections:

```
Bus.Publish ──► Schedule ──► Job ──► Apply ──► Projection
```

### Step by step

**1. A schedule subscribes to events.** There are two scheduling strategies:

- **`schedule.Continuously(bus, store, names)`** — subscribes to the event bus for real-time, event-driven updates. Supports debouncing to batch rapid events into a single job.
- **`schedule.Periodically(store, interval, names)`** — polls the event store at fixed intervals. Better for batch reporting or when real-time updates aren't needed.

**2. The schedule creates a Job.** Each trigger (event received or timer tick) creates a `projection.Job` that wraps access to the event store. The job provides methods to query events:

```go
errs, err := s.Subscribe(ctx, func(job projection.Job) error {
	events, errs, err := job.Events(job)  // fetch relevant events
	// ... or
	job.Apply(job, &myProjection)         // auto-apply to a target
})
```

**3. Events are applied to the projection.** `Apply` walks through events, checks the projection's `Guard` (if any) to filter, checks `Progressor` progress to skip already-processed events, calls `ApplyEvent` on the target, and updates the progress marker.

## Key Interfaces

These five interfaces define the boundaries between components. Any implementation that satisfies them works with the rest of the framework.

### `event.Store`

```go
type Store interface {
	Insert(context.Context, ...Event) error
	Find(context.Context, uuid.UUID) (Event, error)
	Query(context.Context, Query) (<-chan Event, <-chan error, error)
	Delete(context.Context, ...Event) error
}
```

Implementations: [In-Memory](/backends/in-memory), [MongoDB](/backends/mongodb), [PostgreSQL](/backends/postgres).

### `event.Bus`

```go
type Bus interface {
	Publish(ctx context.Context, events ...Event) error
	Subscribe(ctx context.Context, names ...string) (<-chan Event, <-chan error, error)
}
```

Implementations: [In-Memory](/backends/in-memory), [NATS](/backends/nats).

### `command.Bus`

```go
type Bus interface {
	Dispatch(context.Context, Command, ...DispatchOption) error
	Subscribe(ctx context.Context, names ...string) (<-chan Context, <-chan error, error)
}
```

See [Commands](/guide/commands) for details.

### `aggregate.Repository`

```go
type Repository interface {
	Save(ctx context.Context, a Aggregate) error
	Fetch(ctx context.Context, a Aggregate) error
	FetchVersion(ctx context.Context, a Aggregate, v int) error
	Use(ctx context.Context, a Aggregate, fn func() error) error
	Query(ctx context.Context, q Query) (<-chan History, <-chan error, error)
	Delete(ctx context.Context, a Aggregate) error
}
```

See [Repositories](/guide/repositories) for details.

### `codec.Encoding`

```go
type Encoding interface {
	Marshal(any) ([]byte, error)
	Unmarshal([]byte, string) (any, error)
}
```

See [Codec Registry](/guide/codec) for details.

## The Streaming Pattern

goes returns query results and subscriptions as Go channels rather than slices:

```go
events, errs, err := store.Query(ctx, query.New(...))
```

This triple-return pattern — `(<-chan T, <-chan error, error)` — appears throughout the framework. The third value is a setup error (e.g., invalid query). The two channels deliver results and stream-level errors asynchronously.

### Why channels?

- **Memory efficiency** — events are processed one at a time instead of loading thousands into memory
- **Backpressure** — the consumer controls the pace; slow consumers don't cause unbounded buffering
- **Composability** — channels work with `select`, `context.Done()`, and timeouts out of the box
- **Cancellation** — canceling the context stops the stream immediately

### Consuming streams

The `helper/streams` package provides helpers for working with channels. Prefer these over hand-written `select` loops.

**`Walk`** is the go-to for processing a dual-channel stream. It drains both channels, stops on the first error, and respects context cancellation:

```go
events, errs, err := store.Query(ctx, query.New(...))
if err != nil {
	return err
}

err = streams.Walk(ctx, func(evt event.Event) error {
	// process evt
	return nil
}, events, errs)
```

**`Drain`** and **`All`** collect a full stream into a slice:

```go
events, errs, err := store.Query(ctx, query.New(...))
all, err := streams.Drain(ctx, events, errs)  // or streams.All(events, errs) with background context
```

**`Take`** receives exactly *n* elements:

```go
first5, err := streams.Take(ctx, 5, events, errs)
```

**`Await`** returns the next single element or error, whichever comes first:

```go
next, err := streams.Await(ctx, events, errs)
```

**`Filter`** and **`Map`** transform streams:

```go
highPriority := streams.Filter(events, func(e event.Event) bool {
	return e.AggregateName() == "order"
})

names := streams.Map(ctx, events, func(e event.Event) string {
	return e.Name()
})
```

**`FanIn`** merges multiple channels into one; **`FanInContext`** ties the merged channel to a context:

```go
merged := streams.FanInContext(ctx, chanA, chanB, chanC)
```

**`New`** creates a channel from a slice — useful in tests:

```go
ch := streams.New([]event.Event{evt1, evt2, evt3})
```

Under the hood, these helpers run the `select` loop that drains both channels correctly. You can write this loop yourself when you need full control:

```go
for events != nil || errs != nil {
	select {
	case evt, ok := <-events:
		if !ok {
			events = nil
			break
		}
		// process evt
	case err, ok := <-errs:
		if !ok {
			errs = nil
			break
		}
		return err
	}
}
```

## Backend Abstraction

Every backend implements the same interfaces, so swapping is a one-line change:

```go
// Development: in-memory, zero dependencies
var store event.Store = eventstore.New()
var bus event.Bus = eventbus.New()

// Production: MongoDB + NATS
var store event.Store = mongostore.New(client, mongostore.Database("myapp"))
var bus event.Bus = natsbus.New(conn)

// Both use the same decorator, repository, and projections
store = eventstore.WithBus(store, bus)
repo := repository.New(store)
```

Application code never imports a backend package directly — it depends only on the `event.Store` and `event.Bus` interfaces. See [Backends](/backends/) for setup details.
