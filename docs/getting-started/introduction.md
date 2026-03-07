# Introduction

**goes** is an event-sourcing framework for Go. It provides the building blocks for modeling your domain with [aggregates](/guide/aggregates), persisting state as [events](/guide/events), building read models with [projections](/guide/projections), and coordinating workflows across aggregates.

## What is Event Sourcing?

In a traditional application, you store the *current state* of your entities in a database. When something changes, you overwrite the old state with the new state.

With event sourcing, instead of storing current state, you store the *sequence of events* that led to the current state. To get the current state, you replay the events from the beginning.

```
Traditional:  UPDATE accounts SET balance = 900 WHERE id = '...'

Event Sourced: INSERT event (name: 'account.debited', data: {amount: 100})
               → Current state is derived by replaying all events
```

This gives you a complete audit trail, the ability to rebuild state at any point in time, and natural integration points for other systems through event subscriptions.

## What is DDD?

Domain-Driven Design (DDD) is an approach to software development that focuses on modeling your code around business concepts. Key patterns used in goes:

- **[Aggregates](/guide/aggregates)** — Consistency boundaries that protect business rules. They encapsulate state and validate changes before accepting them.
- **[Events](/guide/events)** — Facts about what happened. Past-tense, immutable records of state changes.
- **[Commands](/guide/commands)** — Requests to do something. They express intent and are validated by aggregates before producing events.
- **[Projections](/guide/projections)** — Read-optimized views derived from events. Tailored for specific query needs, updated reactively as events flow through the system.

## Why goes?

- **Generic where it matters** — Typed events, commands, and repositories reduce boilerplate and catch mistakes at compile time.
- **Backend-agnostic** — Swap between [MongoDB](/backends/mongodb), [PostgreSQL](/backends/postgres), [NATS](/backends/nats), or [in-memory backends](/backends/in-memory) without changing application code.
- **Streaming-first APIs** — Queries and subscriptions return Go channels, so you can process large event sets incrementally with low memory overhead.
- **Production-ready** — Built-in support for [snapshots](/guide/snapshots), optimistic concurrency, and continuous [projections](/guide/projections).
- **Minimal boilerplate** — Define your aggregate, register event handlers, and the framework handles versioning, persistence, and replay.

## Streaming by Default

goes works in a streaming manner. Instead of returning large slices for queries and subscriptions, framework APIs typically return result and error channels:

```go
events, errs, err := store.Query(ctx, q)
```

That lets you consume data as it arrives, which keeps memory usage low and avoids waiting for a full result set before doing work. It also fits naturally with Go's `context` cancellation and `select`-based concurrency.

See [The Streaming Pattern](/reference/architecture#the-streaming-pattern) for the full model and helper utilities.

## Next Steps

- [Install goes](/getting-started/installation) and set up your project
- [Quick Start](/getting-started/quick-start) — a minimal working example
- [Tutorial](/tutorial/) — build a full e-commerce application step by step
