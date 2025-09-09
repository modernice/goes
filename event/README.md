# Events

Package `event` contains the primitives used throughout goes. It models
domain events, provides helpers to build and transform them and defines the
interfaces that event buses and stores have to implement.

## Creating events

An event carries a name, a unique identifier, a timestamp and arbitrary data.
Use `event.New` to construct one:

```go
import "github.com/modernice/goes/event"

func example() {
    evt := event.New("foo", 3)
    _ = evt.ID()   // uuid.UUID
    _ = evt.Name() // "foo"
    _ = evt.Time() // time.Time
    _ = evt.Data() // 3
}
```

Events can optionally belong to an aggregate. Pass `event.Aggregate` when
creating an event to associate it with an aggregate ID, name and version.

## Event buses

Publish events by passing them to an implementation of `event.Bus`:

```go
func example(bus event.Bus) error {
    evt := event.New("foo", 3)
    return bus.Publish(context.TODO(), evt)
}
```

Subscribers receive events via channels:

```go
func example(bus event.Bus) {
    events, errs, err := bus.Subscribe(context.TODO(), "foo", "bar")
    if err != nil {
        // handle subscription error
    }
    _ = events // <-chan event.Event
    _ = errs   // <-chan error
}
```

## Event stores

Persist and query events through an implementation of `event.Store`:

```go
func example(store event.Store) {
    evt := event.New("foo", 3)
    _ = store.Insert(context.TODO(), evt)

    q := query.New(
        query.Name("foo"),
        query.SortByTime(),
    )
    events, errs, _ := store.Query(context.TODO(), q)
    _ = events
    _ = errs
}
```

## Encoding event data

Some buses and stores need to encode event data. Use a registry to register
known event names and their Go types:

```go
enc := codec.Gob(event.NewRegistry())
codec.GobRegister[int](enc, "foo")
codec.GobRegister[string](enc, "bar")
```

The encoder can then be supplied to the bus or store implementation.

