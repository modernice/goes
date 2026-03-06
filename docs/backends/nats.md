# NATS

The NATS backend provides a distributed event bus with support for NATS Core and NATS JetStream.

```go
import "github.com/modernice/goes/backend/nats"
```

> For a step-by-step setup, see the [Tutorial](/tutorial/10-backends).

## Event Bus

```go
bus := nats.NewEventBus(enc,
	nats.URL("nats://localhost:4222"),
)
```

The first argument is a `codec.Encoding`. By default, the bus uses the NATS Core driver — simple pub/sub with no persistence.

### Options

| Option | Default | Description |
| --- | --- | --- |
| `URL(string)` | `$NATS_URL`, then `nats.DefaultURL` | NATS server URL |
| `Conn(*nats.Conn)` | — | Reuse an existing NATS connection |
| `Use(Driver)` | `Core()` | Choose Core or JetStream driver |
| `EatErrors()` | off | Silently discard subscription errors |
| `QueueGroup(fn)` | — | Function returning queue group name per event |
| `LoadBalancer(string)` | — | Shorthand for queue group with service name |
| `SubjectFunc(fn)` | identity | Custom NATS subject mapping |
| `SubjectPrefix(string)` | — | Prefix all NATS subjects |
| `PullTimeout(duration)` | 0 (no timeout) | Max time to push an event to a subscriber channel |

URL resolution: explicit `URL()` > `NATS_URL` environment variable > `nats.DefaultURL` (`nats://127.0.0.1:4222`).

## Core vs. JetStream

This is the most important decision when configuring the NATS backend.

**NATS Core** is the default driver. It provides simple publish/subscribe with at-most-once delivery. If no subscriber is listening when an event is published, that event is not delivered through the bus. However, events are always persisted in the event store regardless of bus delivery — they are never truly lost. Projections using `projection.Startup()` catch up from the event store on restart, so Core is sufficient for most applications.

**NATS JetStream** adds persistent streams on top of Core. Events are stored in a stream and can be replayed. Delivery is at-least-once — subscribers receive events even if they were offline when the event was published. Consumers can be durable, meaning they resume from where they left off after a restart.

```go
// Core (default) — simple pub/sub:
bus := nats.NewEventBus(enc)

// JetStream — persistent streams, durable consumers:
bus := nats.NewEventBus(enc,
	nats.Use(nats.JetStream()),
)
```

Core is the simpler choice and works well for most applications. JetStream is useful when you need guaranteed bus delivery — for example, when a subscriber goes offline temporarily and must receive the events it missed without a full restart.

## Connection

The event bus connects lazily on the first `Publish` or `Subscribe` call. To connect explicitly:

```go
err := bus.Connect(ctx)
```

After connecting:

```go
bus.Connection()    // *nats.Conn
bus.Disconnect(ctx) // close gracefully
```

If you already have a NATS connection, pass it with `Conn(conn)` to skip automatic connection.

## JetStream Options

```go
bus := nats.NewEventBus(enc,
	nats.Use(nats.JetStream(
		nats.StreamName("myapp"),
		nats.Durable("myapp"),
	)),
)
```

| Option | Default | Description |
| --- | --- | --- |
| `StreamName(string)` | `"goes"` | Name of the JetStream stream |
| `Durable(string)` | — | Make consumers durable with a name prefix |
| `DurableFunc(fn)` | — | Custom durable name function |

### Stream Auto-Creation

The JetStream driver creates a stream (default name `"goes"`) with a `"*"` subject filter if it does not already exist. If the stream exists with a different configuration, an error is returned.

### Durable Consumers

Without `Durable`, consumers are ephemeral — JetStream creates and destroys them automatically. Ephemeral consumers only receive events published after the subscription starts.

With `Durable(prefix)`, consumers persist across restarts and receive all events from the stream, including those published while the consumer was offline. Durable names are formatted as `prefix:queue:event`, with characters `.`, `*`, `>` replaced by `_`.

```go
// Ephemeral — receives new events only:
nats.Use(nats.JetStream())

// Durable — receives all events, survives restarts:
nats.Use(nats.JetStream(
	nats.Durable("order-service"),
))
```

## Queue Groups and Load Balancing

When running multiple instances of a service, each instance subscribes to the same events. Without queue groups, every instance receives every event — which is wasteful for projections that write to a shared database.

`LoadBalancer` is the simplest way to fix this:

```go
bus := nats.NewEventBus(enc,
	nats.LoadBalancer("order-service"),
)
```

This creates queue groups named `order-service:eventName` so that only one instance receives each event. For more control, use `QueueGroup` directly:

```go
bus := nats.NewEventBus(enc,
	nats.QueueGroup(func(eventName string) string {
		return "my-service"
	}),
)
```

::: warning
Do **not** use a load-balanced bus for the command bus. Commands must reach a specific handler, not be distributed across instances. Create a separate bus without queue groups for `cmdbus.New`.

For projections, think carefully. In-memory projections (like lookup tables) need every instance to receive events. Database-backed projections that use `repo.Use` (fetch-modify-save) can benefit from load balancing.
:::

## Subject Mapping

By default, the event name is used as the NATS subject directly. To add a prefix:

```go
bus := nats.NewEventBus(enc,
	nats.SubjectPrefix("events."),
)
// "shop.product.created" → "events.shop.product.created"
```

For full control over subject mapping:

```go
bus := nats.NewEventBus(enc,
	nats.SubjectFunc(func(eventName string) string {
		return "myapp.events." + eventName
	}),
)
```

## Error Handling

`Subscribe` returns an error channel alongside the events channel:

```go
events, errs, err := bus.Subscribe(ctx, "product.created")
```

Errors on this channel include decoding failures and push timeouts. Use `EatErrors()` to discard them silently when you don't need the error channel.

`PullTimeout(duration)` sets a maximum time for pushing an event to the subscriber's channel. If the subscriber is too slow, the event is dropped and an error is sent to the error channel. Without a pull timeout, a slow subscriber blocks event delivery for that subscription.

```go
bus := nats.NewEventBus(enc,
	nats.PullTimeout(5 * time.Second),
)
```

## Event Encoding

Events are serialized using a gob-encoded envelope containing the event's ID, name, time, data, and aggregate metadata. The `Data` field is encoded and decoded using the `codec.Encoding` passed to `NewEventBus`.

This means NATS messages are binary, not human-readable JSON. If you need to inspect messages, use the goes codec to decode them.

For JetStream, the event's UUID is used as the NATS message ID, which enables JetStream's built-in deduplication.

## Docker

```yaml
services:
  nats:
    image: nats:2
    ports:
      - "4222:4222"
      - "8222:8222"  # monitoring
    command: ["--js"]  # enable JetStream
```

The `--js` flag enables JetStream. Without it, only Core pub/sub is available. Port 8222 exposes the NATS monitoring HTTP endpoint.

See the [Tutorial](/tutorial/10-backends) for a complete Docker Compose setup with MongoDB and NATS.
