# Aggregates

The `Aggregate` module builds on top of [Events](./events.md) and provides
the toolkit to create and manage event-sourced Aggregates.

An Aggregate is a structure that provides at least the following properties:

- aggregate name/type
- aggregate id
- current version
- changes (new events)

An Aggregate can be reconstructed from its Events by reapplying the Events on a
freshly instantiated Aggregate of that type. Here's an example (note that the
`person` Aggregate embeds the aggregate.Base type):

```go
type person struct {
  aggregate.Base

  Name string
  Gender string
}

type bornEvent struct {
  Name string
  Gender string
}

type birthdayEvent struct {}

func (e example) ApplyEvent(evt event.Event) {
  switch evt.Name() {
    case "born":
      data := evt.Data().(bornEvent)
      e.Name = data.Name
      e.Gender = data.Gender
    case "birthday":
      e.Age++
  }
}
```

In the above example the `ApplyEvent` method is responsible for (re)-applying
Events on the Aggregate to reconstruct the current state of the Aggregate.

## Aggregate Repository

The `Aggregate Repository` builds on top of the
[Event Store](./events.md#event-store). It saves and fetches Aggregates to and
from the database as a series of Events.

### Example

```go
type example struct {
  aggregate.Base
}

func (e example) SomeMethod() error {
  // validate action
  // maybe return err

  // create event and add to changes
  e.newEvent("some-event", event.New(...))

  return nil
}

// make event store
store, err := mongostore.New(...)
// handle err

// pass event store to aggregate repository
repo := aggregate.NewRepository(store)

// fetch an aggregate named "foo" with the specified id
a, err := repo.Fetch(context.TODO(), "foo", uuid.New())
// handle err

// cast aggregate interface to concrete type
ea := a.(example)

err = ea.SomeMethod()
// handle err

// save aggregate
err = repo.Save(context.TODO(), ea)
// handle err
```

### Requirements

- save aggregates
- fetch aggregates
- validate aggregate integrity (optimistic concurrency)
