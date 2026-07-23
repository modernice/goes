# MongoDB backend

MongoDB implementations of the goes event store, snapshot store, and model
repository. For setup and usage, see the
[MongoDB backend documentation](https://goes.modernice.dev/backends/mongodb).

Since `goes@v0.9.0`, this backend uses version 2 of the official MongoDB Go
driver:

```text
go.mongodb.org/mongo-driver/v2
```

Applications that exchange MongoDB driver types with this backend — clients,
options, ObjectIDs, sessions — or implement its BSON codec interfaces must
migrate their own driver dependency and imports at the same time as they
upgrade `goes`.

## Migrating from mongo-driver v1

> A published version of this guide, which also covers the other changes in
> `goes@v0.9.0`, is available at
> [goes.modernice.dev/migrations/v0-9](https://goes.modernice.dev/migrations/v0-9).

**No data migration is required.** The backend keeps its database names,
collections, indexes, and stored BSON fields, so existing data remains fully
usable after the driver upgrade.

That storage compatibility does not make application-defined BSON codecs
source-compatible. Custom value codec method signatures must still be updated
as described below.

> **Tip:** This migration is well-suited for an AI coding agent. Give the agent
> this README and the MongoDB migration guide linked below, then ask it to
> update the driver dependency and affected APIs, search for custom BSON codec
> implementations, run `go mod tidy`, compile all code behind the `mongo` build
> tag, run the test suite, and review the resulting diff.

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

### 4. Update custom BSON value codecs

Driver v2 changed the `bson.ValueMarshaler` and `bson.ValueUnmarshaler`
interfaces to represent the BSON type as a `byte`:

```go
type ValueMarshaler interface {
	MarshalBSONValue() (typ byte, data []byte, err error)
}

type ValueUnmarshaler interface {
	UnmarshalBSONValue(typ byte, data []byte) error
}
```

In v1 these methods used `bsontype.Type`. Change the signatures to `byte`, then
convert at calls that still use `bson.Type`:

```go
func (v Value) MarshalBSONValue() (byte, []byte, error) {
	typ, data, err := bson.MarshalValue(v.String())
	return byte(typ), data, err
}

func (v *Value) UnmarshalBSONValue(typ byte, data []byte) error {
	return bson.UnmarshalValue(bson.Type(typ), data, &v.value)
}
```

> **Warning:** Do not use `bson.Type` in these method signatures. Although its
> underlying type is `byte`, it is a distinct named type, so the methods no
> longer implement the driver interfaces. Code can still compile while the
> driver silently falls back to its default encoding or decoding behavior.

Add compile-time assertions so future driver upgrades cannot silently disable
the custom codec:

```go
var (
	_ bson.ValueMarshaler   = Value{}
	_ bson.ValueUnmarshaler = (*Value)(nil)
)
```

Test both the exact stored BSON type and decoding data in its existing on-disk
shape. A round-trip test alone can miss this regression when the default codec
can encode and decode the same unintended representation.

### 5. Update sessions and transaction callbacks

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

### 6. Update query option interceptors

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

### 7. Decode `Distinct` results when using the driver directly

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
- Update every `bson.ValueMarshaler` and `bson.ValueUnmarshaler` implementation
  to use `byte`, add compile-time interface assertions, and verify the stored
  BSON type and decoding of existing data.
- Change stored or returned sessions to `*mongo.Session`.
- Replace `mongo.SessionContext` callback parameters with `context.Context`
  and use `mongo.SessionFromContext`.
- Update `WithQueryOptions` callbacks to use `*options.FindOptionsBuilder`.
- Run `go mod tidy`, then compile and test code behind the `mongo` build tag.
