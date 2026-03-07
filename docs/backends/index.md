# Backends

goes is backend-agnostic. Your application code uses interfaces (`event.Store`, `event.Bus`), and you plug in concrete implementations at startup.

## Available Backends

| Component | Backend | Package |
| --- | --- | --- |
| Event Store | [MongoDB](/backends/mongodb) | `backend/mongo` |
| Event Store | [PostgreSQL](/backends/postgres) | `backend/postgres` |
| Event Store | [In-Memory](/backends/in-memory) | `event/eventstore` |
| Event Bus | [NATS](/backends/nats) | `backend/nats` |
| Event Bus | [In-Memory](/backends/in-memory) | `event/eventbus` |
| Snapshot Store | [MongoDB](/backends/mongodb) | `backend/mongo` |
| Model Repository | [MongoDB](/backends/mongodb) | `backend/mongo` |
| Model Repository | [In-Memory](/backends/in-memory) | `backend/memory` |

## Choosing a Backend

**For development and testing:** Use the [in-memory event store and bus](/backends/in-memory). No external dependencies, instant setup.

**For production:** Use [MongoDB](/backends/mongodb) or [PostgreSQL](/backends/postgres) for the event store (persistent, queryable) and [NATS](/backends/nats) for the event bus (distributed, reliable).

## Backend Guides

- [MongoDB](/backends/mongodb) — event store, snapshot store, model repository
- [PostgreSQL](/backends/postgres) — event store
- [NATS](/backends/nats) — event bus with Core and JetStream
- [In-Memory](/backends/in-memory) — for testing and prototyping
