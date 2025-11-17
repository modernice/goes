# Event Store

An event store persists events in some kind of storage backend. This is mainly
used for the persistence of aggregates, which consist of streams of events, but
can also be used to persist any kind of event that implements `event.Of[T]`.

## Inserting Events

```go
package example

import (
  "context"
  "fmt"
  "github.com/modernice/goes/event"
  "github.com/modernice/goes/event/eventstore"
)

func example() {
  // Create an in-memory event store.
  store := eventstore.New()

  insert(store)
}

func insert(store event.Store) {
  events := []event.Event{
    event.New("foo", "foo-data").Any(),
    event.New("bar", "bar-data").Any(),
  }

  if err := store.Insert(context.TODO(), events...); err != nil {
    panic(fmt.Errorf("insert events: %w", err))
  }
}
```

As you might noticed, in the above `insert` function, when creating the events,
their `Any()` methods are called. This is necessary in this case because of Go's
current limitations in type parameters (generics). The `Insert()` method of the
`event.Store` interface expects the `event.Event / event.Of[any]` type, but when
creating an event with for example a `string` as event data, the created type
will be `event.Of[string]`. The `Any()` method of an event returns the event as
an `event.Event / event.Of[any]`.

### Inserting Aggregate Events

When inserting events of an aggregate into an event store, the store will
perform additional checks to ensure the consistency of the inserted events.
For example, when inserting two events with the same position in an aggregate's
event stream, the insertion will fail for all events, effectively providing
Optimistic Concurrency Control for free.

```go
package example

import (
  "context"
  "fmt"
  "github.com/modernice/goes/internal"
  "github.com/modernice/goes/event"
)

func example(store event.Store) {
  id := internal.NewUUID()
  name := "example"
  version := 1

  // We have two events with the same Aggregate Version.
  events := []event.Event{
    event.New("foo", "foo-data", event.Aggregate(id, name, version)).Any(),
    event.New("bar", "bar-data", event.Aggregate(id, name, version)).Any(),
  }

  // This will fail due to the consistency check.
  if err := store.Insert(context.TODO(), events...); err != nil {
    panic(fmt.Errorf("insert events: %w", err))
  }
}
```

## Querying Events

The `event.Store` provides sophisticated [query capabilities](#type-definitions)
that allow you to query events by any of the following fields:

- `ID`
- `Name`
- `Time`
- `AggregateID`
- `AggregateName`
- `AggregateVersion`

### Example

```go
package example

import (
  stdtime "time"
  "github.com/modernice/goes/event"
  "github.com/modernice/goes/event/query"
  "github.com/modernice/goes/event/query/time"
  "github.com/modernice/goes/event/query/version"
)

func example(store event.Store) {
  events, errs, err := store.Query(context.TODO(), query.New(
    // Filter by event id.
    query.ID(uuid.UUID{...}, uuid.UUID{...}),

    // Filter by event name.
    query.Name("foo", "bar", "baz"),

    // Filter by event time.
    query.Time(
      // Allow exact matches.
      time.Exact(stdtime.Time{...}, stdtime.Time{...}),

      // Allow range matches.
      time.InRange(
        time.Range{stdtime.Time{...}, stdtime.Time{...}},
        time.Range{stdtime.Time{...}, stdtime.Time{...}},
      ),

      // Allow events that happened before the given time.
      time.Before(stdtime.Time{...}),

      // Allow events that happened after the given time.
      time.After(stdtime.Time{...}),

      // Same as time.After() but "inclusive".
      time.Min(stdtime.Time{...}),

      // Same as time.Before() but "inclusive".
      time.Max(stdtime.Time{...}),
    ),

    // Filter by aggregate id.
    query.AggregateID(uuid.UUID{...}, uuid.UUID{...}),

    // Filter by aggregate name.
    query.AggregateName("foo", "bar", "baz"),

    // Filter by aggregate version.
    query.AggregateVersion(
      // Allow exact matches.
      version.Exact(1, 4, 6),

      // Allow range matches.
      version.InRange(
        time.Range{1, 4}, // between version 1 and 4
        time.Range{10, 12}, // between version 10 and 12
      ),

      // Allow events with AggregateVersion >= 4.
      version.Min(4),

      // Allow events with AggregateVersion <= 12.
      version.Max(12),
    ),

    // Allow events that belong to a specific aggregate.
    query.Aggregate("foo", uuid.UUID{...}),

    // Now, the query allows events that belong to the above "foo" aggregate
    // OR events that belong to the below "bar" aggregate.
    query.Aggregate("bar", uuid.UUID{...}),

    // Allow all events that belong to "foo" aggregates.
    // Basically the same as query.AggregateName("foo")
    query.Aggregate("foo", uuid.Nil),
  ))
}
```

### Sorting

When querying events, you can specify how the result should be sorted.

#### Simple sorting

Simple sorting means that the result is sorted by a single field.

```go
package example

import (
  "context"
  "fmt"
  "github.com/modernice/goes/event"
  "github.com/modernice/goes/event/query"
)

func example(store event.Store) {
  events, errs, err := store.Query(context.TODO(), query.New(
    query.Name("foo", "bar", "baz"),

    // Sort events by time, ascending.
    query.SortBy(event.SortTime, event.SortAsc),

    // Sort events by aggregate name, descending.
    query.SortBy(event.SortAggregateName, event.SortDesc),

    // Sort events by aggregate id, ascending.
    query.SortBy(event.SortAggregateID, event.SortAsc),

    // Sort events by aggregate version, descending.
    query.SortBy(event.SortAggregateVersion, event.SortDesc),
  ))

  // Drain the stream to get a slice of events.
  all, err := streams.Drain(context.TODO(), events, errs)
  if err != nil {
    panic(fmt.Errorf("drain events: %w", err))
  }
}
```

#### Multi-sorting

You can also sort by multiple fields, assuming that the event store
implementation supports multi-field sorting.

```go
package example

import (
  "context"
  "fmt"
  "github.com/modernice/goes/event"
  "github.com/modernice/goes/event/query"
)

func example(store event.Store) {
  events, errs, err := store.Query(context.TODO(), query.New(
    query.Name("foo", "bar", "baz"),

    // Sort events first by aggregate name, then aggregate id,
    // and then by aggregate version.
    query.SortByMulti(
      event.SortOptions{
        Sort: event.SortAggregateName,
        Dir: event.SortAsc,
      },
      event.SortOptions{
        Sort: event.SortAggregateID,
        Dir: event.SortAsc,
      },
      event.SortOptions{
        Sort: event.SortAggregateVersion,
        Dir: event.SortAsc,
      },
    ),
  ))

  // Drain the stream to get a slice of events.
  all, err := streams.Drain(context.TODO(), events, errs)
  if err != nil {
    panic(fmt.Errorf("drain events: %w", err))
  }
}
```

### Fetching Events

If you already know the ID of an event, you can directly fetch the event from
the event store.

```go
package example

import (
  "context"
  "fmt"
  "github.com/modernice/goes/event"
  "github.com/modernice/goes/event/query"
)

func example(store event.Store) {
  id := internal.NewUUID()

  evt, err := store.Find(context.TODO(), id)
  if err != nil {
    panic(fmt.Errorf("event %q not found: %w", id, err))
  }
}
```

## Deleting Events

::: danger
When deleting an event via the `event.Store.Delete()` method, the event will be
irrevocably deleted from the event store. Doing this requires careful
consideration of the application structure in order to avoid unexpected issues
caused by missing events, especially within an event-sourced application.
**When you delete an event from the event store, make sure that your application
does not depend on that event to function properly.**
:::

::: tip
Consider using the [soft-delete feature](/guide/aggregates/soft-deletes)
when deleting aggregates. Soft-deletes allow you to mark aggregates as deleted
without actually deleting events from the event store.
:::

```go
package example

import (
  "context"
  "fmt"
  "github.com/modernice/goes/event"
  "github.com/modernice/goes/event/query"
)

func example(store event.Store) {
  id := internal.NewUUID()

  evt, err := store.Find(context.TODO(), id)
  if err != nil {
    panic(fmt.Errorf("event %q not found: %w", id, err))
  }

  if err := store.Delete(context.TODO(), evt); err != nil {
    panic(fmt.Errorf("delete %q event: %w", evt.Name(), err))
  }
}
```

## Type Definitions

### Query

<<< @/../../event/store.go#query

### Sortings

<<< @/../../event/store.go#sortings
