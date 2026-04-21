# natsjs

> **Experimental** -- This backend is not tested in production. The API may
> change without notice.

Combined `event.Store` and `event.Bus` backed by NATS JetStream.

## How it works

Events are stored in **per-aggregate-type JetStream streams** (created lazily
on first insert) and indexed in a **KV bucket** for O(1) UUID lookups. The
single `EventStore` type implements both `event.Store` and `event.Bus`.

### Subjects

```
<namespace>.<aggregateName>.<aggregateID>.<eventName>
```

Examples (default namespace `goes`):

```
goes.orders.550e8400-e29b-41d4-a716-446655440000.order_placed
goes._._.user_logged_in
```

### JetStream resources

| Resource | Naming | Example |
|---|---|---|
| Aggregate stream | `<namespace>_agg_<aggregateName>` | `goes_agg_orders` |
| Non-aggregate stream | `<namespace>_agg__` | `goes_agg__` |
| KV bucket | configurable | `goes_idx` |

## Usage

```go
store := natsjs.NewEventStore(enc)

// Explicit connect (optional -- called automatically by Insert/Find/Query/etc.)
if err := store.Connect(ctx); err != nil {
    log.Fatal(err)
}
defer store.Disconnect(ctx)
```

### Options

```go
store := natsjs.NewEventStore(enc,
    natsjs.URL("nats://localhost:4222"),      // or set NATS_URL env var
    natsjs.Conn(existingConn),                // use an existing *nats.Conn
    natsjs.Namespace("myapp"),                // default: "goes"
    natsjs.KVBucket("myapp_idx"),             // default: "goes_idx"
    natsjs.LoadBalancer("order-service"),      // enable load-balanced subscriptions
)
```

### Load balancing

By default, every subscriber instance receives every event. Use `LoadBalancer`
to distribute events across instances of the same service:

```go
store := natsjs.NewEventStore(enc,
    natsjs.LoadBalancer("order-service"),
)
```

Instances sharing the same service name create shared durable JetStream
consumers, so each event is delivered to exactly one instance.
