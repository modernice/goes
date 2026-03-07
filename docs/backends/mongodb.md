# MongoDB

The MongoDB backend provides an event store, a snapshot store, and a model repository.

```go
import "github.com/modernice/goes/backend/mongo"
```

> For a step-by-step setup, see the [Tutorial](/tutorial/11-backends).

## Event Store

```go
store := mongo.NewEventStore(enc,
	mongo.URL("mongodb://localhost:27017"),
	mongo.Database("myapp"),
)
```

The first argument is a `codec.Encoding` — the same [codec registry](/guide/codec) used throughout goes. The event store connects lazily on the first operation; you don't need to call `Connect` explicitly.

Defaults: database `"event"`, events collection `"events"`, aggregate state collection `"states"`.

### Options

| Option | Default | Description |
| --- | --- | --- |
| `URL(string)` | `$MONGO_URL` | MongoDB connection string |
| `Client(*mongo.Client)` | — | Reuse an existing `*mongo.Client` |
| `Database(string)` | `"event"` | Database name |
| `Collection(string)` | `"events"` | Events collection name |
| `StateCollection(string)` | `"states"` | Aggregate state collection name |
| `Transactions(bool)` | `false` | Use MongoDB transactions for inserts |
| `ValidateVersions(bool)` | `true` | Check aggregate versions before insert |
| `NoIndex(bool)` | `false` | Skip automatic index creation |
| `WithIndices(...)` | — | Add custom MongoDB index models |
| `WithQueryOptions(fn)` | — | Intercept `FindOptions` on queries |
| `WithTransactionHook(hook, fn)` | — | Run code inside insert transactions |

### Connection

The event store connects to MongoDB automatically on the first `Insert`, `Find`, `Query`, or `Delete` call. To connect explicitly:

```go
client, err := store.Connect(ctx)
```

You can pass additional `*options.ClientOptions` to `Connect`. After connecting, the underlying client and collections are available through accessors:

```go
store.Client()          // *mongo.Client
store.Database()        // *mongo.Database
store.Collection()      // *mongo.Collection (events)
store.StateCollection() // *mongo.Collection (aggregate states)
```

URL resolution: explicit `URL()` option > `MONGO_URL` environment variable.

### Two Collections

The event store uses two MongoDB collections:

- **Events** — stores serialized event documents with fields like `id`, `name`, `time`, `timeNano`, `data`, `aggregateName`, `aggregateId`, `aggregateVersion`.
- **States** — tracks the current version of each aggregate. Before inserting events, the store reads the aggregate's version from this collection and compares it against the incoming events.

The states collection is what powers optimistic concurrency.

### Optimistic Concurrency

By default, the event store validates aggregate versions before inserting events. If another process inserted events for the same aggregate between your fetch and save, the store returns a `VersionError`:

```go
err := store.Insert(ctx, events...)

var versionErr *mongo.VersionError
if errors.As(err, &versionErr) {
	// Another process inserted events first.
	fmt.Printf("expected version %d for %s/%s\n",
		versionErr.CurrentVersion,
		versionErr.AggregateName,
		versionErr.AggregateID,
	)
}
```

`VersionError` implements `IsConsistencyError() bool`, which the repository uses to detect conflicts and retry. Disable validation with `ValidateVersions(false)` if you're doing bulk imports or data migrations.

### Indices

Five core indices are created automatically on the events collection:

| Keys | Unique | Purpose |
| --- | --- | --- |
| `id` | Yes | Event lookup by ID |
| `name` | No | Filter by event name |
| `name`, `timeNano` | No | Time-ordered queries per event type |
| `aggregateName`, `aggregateVersion` | No | Aggregate event queries |
| `aggregateName`, `aggregateId`, `aggregateVersion` | Yes | Unique constraint per aggregate version |

Use `NoIndex(true)` to skip index creation if you manage indices externally. To add additional indices:

```go
import "github.com/modernice/goes/backend/mongo/indices"

store := mongo.NewEventStore(enc,
	mongo.WithIndices(
		indices.EventStore.NameAndVersion,
		indices.EventStore.ISOTime,
	),
)
```

Available edge-case indices: `AggregateID`, `AggregateVersion`, `NameAndVersion`, `ISOTime`.

### Transactions and Hooks

MongoDB transactions require a replica set or sharded cluster. Enable them explicitly or by adding a hook:

```go
store := mongo.NewEventStore(enc,
	mongo.Transactions(true),
)
```

Transaction hooks run code inside the same transaction as the event insert. Two hook points are available: `PreInsert` (before events are written) and `PostInsert` (after events are written but before commit).

```go
store := mongo.NewEventStore(enc,
	mongo.WithTransactionHook(mongo.PostInsert, func(ctx mongo.TransactionContext) error {
		for _, evt := range ctx.InsertedEvents() {
			// Do additional work within the same transaction.
			// Access ctx.Session() for the MongoDB session.
		}
		return nil
	}),
)
```

`WithTransactionHook` automatically enables transactions. If any hook returns an error, the entire transaction is aborted.

You can also extract the transaction from a context using `mongo.TransactionFromContext(ctx)`.

## Snapshot Store

The [snapshot store](/guide/snapshots#snapshot-store) persists aggregate snapshots to avoid replaying the full event history on every fetch.

```go
snapshots := mongo.NewSnapshotStore(
	mongo.SnapshotURL("mongodb://localhost:27017"),
	mongo.SnapshotDatabase("myapp"),
	mongo.SnapshotCollection("snapshots"),
)
```

### Options

| Option | Default | Description |
| --- | --- | --- |
| `SnapshotURL(string)` | `$MONGO_URL` | MongoDB connection string |
| `SnapshotDatabase(string)` | `"snapshot"` | Database name |
| `SnapshotCollection(string)` | `"snapshots"` | Collection name |

### Methods

| Method | Description |
| --- | --- |
| `Save(ctx, snapshot)` | Save or upsert a snapshot |
| `Latest(ctx, name, id)` | Get the most recent snapshot for an aggregate |
| `Version(ctx, name, id, version)` | Get a snapshot at a specific version |
| `Limit(ctx, name, id, version)` | Get the latest snapshot at or below a version |
| `Query(ctx, query)` | Stream snapshots matching a query |
| `Delete(ctx, snapshot)` | Delete a snapshot |

`Latest` is the most common — the repository calls it to skip replaying events that the snapshot already covers. `Limit` is useful when you need state at a specific point in time.

Three indices are created automatically: `goes_time`, `goes_time_nano`, and a unique compound index on `aggregateName + aggregateId + aggregateVersion`.

Wire the snapshot store into a repository with `repository.WithSnapshots(snapshots)`.

## Model Repository

The model repository persists read models ([projections](/guide/projections)) in MongoDB.

Unlike the in-memory model repository, the MongoDB version takes a `*mongo.Collection` directly:

```go
import (
	gomongo "github.com/modernice/goes/backend/mongo"
)

col := eventStore.Client().Database("myapp").Collection("shop_stats")

repo := gomongo.NewModelRepository[*ShopStats, uuid.UUID](col,
	gomongo.ModelFactory(NewShopStats, true),
)
```

The second argument to `ModelFactory` controls what happens when `Fetch` doesn't find a document. With `true`, the factory creates a new model instead of returning an error — useful for singleton projections like ShopStats.

### Options

| Option | Default | Description |
| --- | --- | --- |
| `ModelIDKey(string)` | `"_id"` | BSON field for the model ID |
| `ModelTransactions(bool)` | `false` | Wrap `Use()` in a transaction |
| `ModelFactory(fn, createIfNotFound)` | — | Factory function for creating models |
| `ModelDecoder(fn)` | — | Custom decoder from `*mongo.SingleResult` |
| `ModelEncoder(fn)` | — | Custom encoder for upsert documents |

### Methods

| Method | Description |
| --- | --- |
| `Save(ctx, model)` | Upsert the model |
| `Fetch(ctx, id)` | Fetch a model by ID |
| `Use(ctx, id, fn)` | Fetch, apply function, save — atomically |
| `Delete(ctx, model)` | Delete a model |
| `CreateIndexes(ctx)` | Create a unique index on the ID field |

`Use` is the primary method for projections — it loads the model, runs your function, and saves the result. With `ModelTransactions(true)`, the entire operation runs in a MongoDB transaction.

`CreateIndexes` is only needed when `ModelIDKey` is not `"_id"` (MongoDB auto-indexes `_id`).

Use `ModelDecoder` and `ModelEncoder` when your Go model doesn't map directly to BSON — for example, embedded structs or custom types that need special handling.

## Docker

```yaml
services:
  mongodb:
    image: mongo:8
    ports:
      - "27017:27017"
    volumes:
      - mongo-data:/data/db

volumes:
  mongo-data:
```

For transactions, MongoDB requires a replica set. Add to the service:

```yaml
    command: ["--replSet", "rs0"]
```

Then initialize the replica set:

```bash
mongosh --eval "rs.initiate()"
```

See the [Tutorial](/tutorial/11-backends) for a complete Docker Compose setup with MongoDB and NATS.
