# In-Memory

The in-memory backends are included in the core framework — no additional dependencies needed.

## Event Store

```go
import "github.com/modernice/goes/event/eventstore"

store := eventstore.New()
```

The in-memory event store supports the full `event.Store` interface: insert, find, query, and delete. Events are stored in memory and lost when the process exits.

## Event Bus

```go
import "github.com/modernice/goes/event/eventbus"

bus := eventbus.New()
```

The in-memory event bus supports the full `event.Bus` interface: publish and subscribe. Events are delivered to subscribers within the same process.

## Auto-Publish with `WithBus`

To automatically publish events when they're inserted into the store:

```go
store := eventstore.WithBus(eventstore.New(), bus)
```

## When to Use

- **Testing** — no infrastructure setup, fast execution
- **Prototyping** — get started without databases
- **Development** — iterate quickly on domain logic
- **Single-process applications** — when persistence isn't needed

## Limitations

- Events are lost when the process stops
- No distribution — subscribers must be in the same process
- No persistence — cannot survive restarts
- Not suitable for production (unless you don't need durability)
