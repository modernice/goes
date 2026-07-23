# MongoDB backend

MongoDB implementations of the goes event store, snapshot store, and model
repository. For setup and usage, see the
[MongoDB backend documentation](../../docs/backends/mongodb.md).

Since `goes@v0.9.0`, this backend uses version 2 of the official MongoDB Go
driver:

```text
go.mongodb.org/mongo-driver/v2
```

Applications that exchange MongoDB driver types with this backend — clients,
options, ObjectIDs, sessions — must migrate their own driver dependency and
imports at the same time as they upgrade `goes`.

## Migrating from mongo-driver v1

**No data migration is required.** The backend keeps its database names,
collections, indexes, and stored BSON fields, so existing data remains fully
usable after the driver upgrade.

> **Tip:** This migration is well-suited for an AI coding agent. Give the agent
> this README and the MongoDB migration guide linked below, then ask it to
> update the driver dependency and affected APIs, run `go mod tidy`, compile
> all code behind the `mongo` build tag, run the test suite, and review the
> resulting diff.

MongoDB maintains a comprehensive
[Go Driver 2.0 migration guide](https://github.com/mongodb/mongo-go-driver/blob/master/docs/migration-2.0.md).
The sections below cover the changes most relevant to this backend.

### 1. Upgrade the module and imports

```bash
go get go.mongodb.org/mongo-driver/v2@v2.8.0
go mod tidy
```

Add `/v2` to every driver import:

```go
// Before
import "go.mongodb.org/mongo-driver/mongo"

// After
import "go.mongodb.org/mongo-driver/v2/mongo"
```

The same applies to packages such as `bson`, `mongo/options`, and
`mongo/readpref`.

### 2. Update direct `mongo.Connect` calls

`mongo.Connect` no longer accepts a `context.Context`:

```go
// Before
client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))

// After
client, err := mongo.Connect(options.Client().ApplyURI(uri))
```

This only affects direct calls to the driver. The backend's
`(*EventStore).Connect(ctx, opts...)` and `(*SnapshotStore).Connect(ctx)`
methods still accept a context.

### 3. Replace `primitive.ObjectID` with `bson.ObjectID`

The former `bson/primitive` package was merged into `bson`:

```go
// Before
import "go.mongodb.org/mongo-driver/bson/primitive"

id := primitive.NewObjectID()
var modelID primitive.ObjectID

// After
import "go.mongodb.org/mongo-driver/v2/bson"

id := bson.NewObjectID()
var modelID bson.ObjectID
```

This is especially relevant when `bson.ObjectID` is the ID type of a
`ModelRepository`.

### 4. Update sessions and transaction callbacks

In v2, `mongo.Session` is a struct used through `*mongo.Session`, and session
callbacks receive a standard `context.Context`. Retrieve the session with
`mongo.SessionFromContext`:

```go
err := client.UseSession(ctx, func(ctx context.Context) error {
	session := mongo.SessionFromContext(ctx)

	if err := session.StartTransaction(); err != nil {
		return err
	}

	// Use ctx for operations that belong to the session.
	return session.CommitTransaction(ctx)
})
```

Accordingly, `Transaction.Session()` from this backend now returns
`*mongo.Session`. Transaction hooks still receive `mongo.TransactionContext`.

### 5. Update query option interceptors

Driver v2 represents operation options as builders. A `WithQueryOptions`
callback now receives and returns `*options.FindOptionsBuilder`:

```go
store := mongo.NewEventStore(enc,
	mongo.WithQueryOptions(func(opts *options.FindOptionsBuilder) *options.FindOptionsBuilder {
		return opts.SetBatchSize(500)
	}),
)
```

Code that only constructs options with calls such as
`options.Find().SetLimit(...)` keeps the same shape. Code that reads option
fields must first apply the builder's setters to the corresponding options
value.

### 6. Decode `Distinct` results when using the driver directly

`Collection.Distinct` now returns a result that must be decoded:

```go
var names []string
if err := collection.Distinct(ctx, "name", bson.D{}).Decode(&names); err != nil {
	return err
}
```

## Migration checklist

- Replace every `go.mongodb.org/mongo-driver/...` import with
  `go.mongodb.org/mongo-driver/v2/...`.
- Remove the context argument from direct `mongo.Connect` calls.
- Replace `primitive.ObjectID` and `primitive.NewObjectID` with their `bson`
  equivalents.
- Change stored or returned sessions to `*mongo.Session`.
- Replace `mongo.SessionContext` callback parameters with `context.Context`
  and use `mongo.SessionFromContext`.
- Update `WithQueryOptions` callbacks to use `*options.FindOptionsBuilder`.
- Run `go mod tidy`, then compile and test code behind the `mongo` build tag.
