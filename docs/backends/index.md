# Backends

goes is backend-agnostic. Your application code uses interfaces (`event.Store`, `event.Bus`), and you plug in concrete implementations at startup.

## Available Backends

| Component | Backend | Package |
| --- | --- | --- |
| Event Store | MongoDB | `backend/mongo` |
| Event Store | PostgreSQL | `backend/postgres` |
| Event Store | In-Memory | `event/eventstore` |
| Event Bus | NATS | `backend/nats` |
| Event Bus | In-Memory | `event/eventbus` |
| Snapshot Store | MongoDB | `backend/mongo` |
| Model Repository | MongoDB | `backend/mongo` |
| Model Repository | In-Memory | `backend/memory` |

## Choosing a Backend

**For development and testing:** Use the in-memory event store and event bus. No external dependencies, instant setup.

**For production:** Use MongoDB or PostgreSQL for the event store (persistent, queryable) and NATS for the event bus (distributed, reliable).

## Backend Guides

- [MongoDB](/backends/mongodb) — event store, snapshot store, model repository
- [PostgreSQL](/backends/postgres) — event store
- [NATS](/backends/nats) — event bus with Core and JetStream
- [In-Memory](/backends/in-memory) — for testing and prototyping
