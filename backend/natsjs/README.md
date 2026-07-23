# natsjs

> ⚠️ **Deprecated — do not use.** This backend has major architectural flaws
> and will be removed in a future release.

## Why you should not use this backend

An architecture review found fundamental problems that cannot be fixed without
a redesign, including:

- **No optimistic concurrency control.** Aggregate versions are never
  validated on insert, so concurrent saves of the same aggregate silently
  corrupt its event history.
- **Silently incomplete queries.** Queries without aggregate filters only see
  streams known to the current process, so e.g. catch-up projections in a
  freshly started process can return partial or empty results.
- **Store/bus conflation.** `Publish` is `Insert`, which breaks the standard
  `eventstore.WithBus` wiring (every save fails as a duplicate) and persists
  transient bus events forever.
- **Non-atomic writes.** Events are written to a stream and a KV index in two
  separate steps, so crashes can leave events that are visible to queries and
  subscribers but invisible to `Find`/`Delete` — or half-saved aggregates.
- **Broken load balancing and delivery guarantees.** Load-balanced
  subscriptions fail on restart, split events between independent subscribers,
  and messages are acked before delivery (events can be lost on shutdown).

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
