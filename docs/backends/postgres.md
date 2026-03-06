# PostgreSQL

The PostgreSQL backend provides an event store. It auto-creates the database, table, and indexes on first connection.

```go
import "github.com/modernice/goes/backend/postgres"
```

> For a step-by-step setup, see the [Tutorial](/tutorial/10-backends).

::: tip
The PostgreSQL event store enforces version uniqueness through a database constraint — duplicate aggregate versions are rejected. However, it does not perform proactive version validation like the MongoDB backend, which checks that new events are sequential (`currentVersion + 1`) before inserting.
:::

## Event Store

```go
store := postgres.NewEventStore(enc,
	postgres.URL("postgres://localhost:5432/postgres?sslmode=disable"),
)
```

The first argument is a `codec.Encoding`. The event store connects lazily on the first operation.

### Options

| Option | Default | Description |
| --- | --- | --- |
| `URL(string)` | `$POSTGRES_EVENTSTORE` | PostgreSQL connection string |
| `Database(string)` | `"goes"` | Database name (auto-created if missing) |
| `Table(string)` | `"events"` | Events table name |

### Connection

The event store connects automatically on the first `Insert`, `Find`, `Query`, or `Delete` call. To connect explicitly:

```go
err := store.Connect(ctx)
```

The store uses `pgxpool.Pool` from pgx v4 for connection pooling. After connecting, the underlying pool is available:

```go
store.Pool() // *pgxpool.Pool
```

## Auto-Migration

On first connection, the store runs a 5-step setup:

1. Connects to PostgreSQL at the base URL
2. Creates the database if it does not exist
3. Reconnects targeting the new database
4. Creates the events table if it does not exist
5. Creates indexes

### Table Schema

```sql
CREATE TABLE IF NOT EXISTS events (
	id UUID PRIMARY KEY NOT NULL,
	name VARCHAR(255) NOT NULL,
	time BIGINT NOT NULL,
	aggregate_id UUID,
	aggregate_name VARCHAR(255),
	aggregate_version INTEGER,
	data JSONB
)
```

The `time` column stores nanosecond Unix timestamps as `BIGINT`, not a PostgreSQL `TIMESTAMP`. Event data is stored as `JSONB`.

### Indexes

| Index | Fields | Unique |
| --- | --- | --- |
| `goes_name` | `name` | No |
| `goes_time` | `time` | No |
| `goes_aggregate` | `aggregate_id`, `aggregate_name`, `aggregate_version` | Yes |

The unique `goes_aggregate` index prevents duplicate aggregate versions — this is the only consistency protection currently available.

## Querying

The store supports the full `event.Query` interface — filter by event IDs, names, aggregate names/IDs/versions, and time ranges, with sorting:

```go
import "github.com/modernice/goes/event/query"

events, errs, err := store.Query(ctx, query.New(
	query.AggregateName("shop.product"),
	query.SortBy(event.SortTime, event.SortAsc),
))
```

Results are streamed through channels, suitable for large result sets.

## Transactions

Insert and delete operations use PostgreSQL transactions internally. All events in a single `Insert` call are committed atomically — either all are written or none are.

There is no user-facing transaction hook system like MongoDB's `WithTransactionHook`. If you need to perform additional operations in the same transaction, use the underlying `Pool()` directly.

## What PostgreSQL Does Not Have

- **No proactive version validation** — unlike MongoDB, there is no pre-insert check that event versions are sequential. The unique `goes_aggregate` index catches duplicate versions, but it does not verify that you're inserting version `N+1` after version `N`. MongoDB's `ValidateVersions` and its separate state collection provide this stricter guarantee.
- **No snapshot store** — use the [MongoDB](/backends/mongodb) snapshot store, or implement your own.
- **No model repository** — use the [MongoDB](/backends/mongodb) model repository, the [in-memory](/backends/in-memory) one, or implement your own.

You can combine PostgreSQL for the event store with MongoDB for snapshots and model storage.

## Docker

```yaml
services:
  postgres:
    image: postgres:17
    ports:
      - "5432:5432"
    environment:
      POSTGRES_PASSWORD: postgres
    volumes:
      - pg-data:/var/lib/postgresql/data

volumes:
  pg-data:
```

Connection string: `postgres://postgres:postgres@localhost:5432?sslmode=disable`

The initial connection URL should point to the PostgreSQL server (not a specific database), or to an existing database like `postgres`. The store creates the target database automatically.
