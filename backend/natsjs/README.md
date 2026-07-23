# natsjs

> ⚠️ **Deprecated — do not use.** This backend has major architectural flaws
> and will be removed in a future release.

## Why you should not use this backend

An architecture review found fundamental problems that cannot be fixed without
a redesign.

### Correctness

- **No optimistic concurrency control.** Aggregate versions are never
  validated on insert, so concurrent saves of the same aggregate silently
  corrupt its event history.
- **Process-local stream discovery.** Streams are discovered through
  in-process caches and signals instead of the server. Subscribers never learn
  about aggregate types first created by another process — those events are
  silently never delivered until the subscriber restarts — and queries without
  aggregate filters return partial or empty results depending on what the
  current process happens to have cached.
- **Non-atomic writes.** Events are written to a stream and a KV index in two
  separate steps, so crashes can leave events that are visible to queries and
  subscribers but invisible to `Find`/`Delete` — or half-saved aggregates.
- **Broken load balancing and delivery guarantees.** Load-balanced
  subscriptions fail on restart, split events between independent subscribers,
  and messages are acked before delivery (events can be lost on shutdown).

### Performance and resource use

The design imposes costs that grow with the total amount of stored data:

- **Broker memory grows with business data.** Every event allocates a KV key,
  and every (aggregate, event type) pair allocates a distinct subject.
  JetStream keeps per-subject state in memory, so broker RAM grows linearly
  with the total number of aggregates and events, forever.
- **Writes are bottlenecked on network latency.** Every insert — and thus
  every publish — costs two sequential synchronous round trips, with no
  batching or async publishing.
- **Every aggregate load is a control-plane operation.** Each read creates and
  tears down a server-side consumer, and always fetches the aggregate's full
  history: version filters are applied client-side, so snapshots do not reduce
  the amount of data read.
- **Unfiltered queries scan and buffer everything.** Queries without aggregate
  filters scan all streams, decode every message, and buffer the entire result
  set in memory before emitting a single event.
- **Consumer explosion.** Each subscription creates one server-side consumer
  per (stream × event name), and load-balanced durable consumers are never
  cleaned up.
- **Unbounded retention.** Streams are created without retention limits, so
  disk usage grows with bus traffic, not just with domain history.

## Use instead

- [`backend/mongo`](../mongo) or [`backend/postgres`](../postgres) as the
  `event.Store`
- [`backend/nats`](../nats) as the `event.Bus`

## Cleaning up existing resources

If you experimented with this backend, it created the following JetStream
resources (default namespace `goes`), which you may want to delete:

| Resource | Naming | Example |
|---|---|---|
| Aggregate stream | `<namespace>_agg_<aggregateName>` | `goes_agg_orders` |
| Non-aggregate stream | `<namespace>_agg__` | `goes_agg__` |
| KV bucket | configurable, default `goes_idx` | `goes_idx` |
