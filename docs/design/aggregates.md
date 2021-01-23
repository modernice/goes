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

### Defining Aggregates

```go
type Person struct {
  aggregate.Base

  Name string
  Birthdate time.Time
}

func New(id uuid.UUID) *Person {
  return &Person{
    Base: aggregate.New(id),
  }
}

func NewAggregate(id uuid.UUID) aggregate.Aggregate {
  return New(id)
}

func (p *Person) Age() int {
  // note: wrong implementation; just to demonstrate
  return time.Now().Year() - p.Birthdate.Year()
}
```

### Defining Commands

```go
type Person struct { ... }

type bornData struct {
  Name string
  Birthdate time.Time
}

func (p *Person) Birth(name string) error {
  if !p.Birthdate.IsZero() {
    return fmt.Errorf("%q was already born", p.Name)
  }

  p.event("born", bornData{
    Name: name,
    Birthdate: time.Now(),
  })

  return nil
}
```

### Applying Events

```go
type Person struct { ... }
type bornData struct { ... }

func (p *Person) ApplyEvent(evt event.Event) {
  switch evt.Name() {
  case "born":
    p.born(evt)
  case "xxx":
    p.xxx(evt)
  case "yyy":
    p.yyy(evt)
  case "zzz":
    p.zzz(evt)
  }
}
```

## Aggregate Repository

The `Aggregate Repository` builds on top of the
[Event Store](./events.md#event-store). It saves and fetches Aggregates to and
from the database as a series of Events.

### Example

```go
type example struct {
  aggregate.Aggregate
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

### Fetching Aggregates

```go
type example struct { ... }

repo := repository.New(...)

a, err := repo.Get(context.TODO(), "foo", uuid.New()) // fetch latest
a, err := repo.GetVersion(context.TODO(), "foo", uuid.New(), 8) // fetch specific version
err := repo.Apply(context.TODO(), &example{...}) // fetch remaining events and apply them
err := repo.ApplyVersion(context.TODO(), &example{...}, 15) // fetch events until v15 and apply them
```

### Saving Aggregates

```go
type example struct { ... }

repo := aggregate.NewRepository(...)

ex := NewExample(...)

err := repo.Save(context.TODO(), ex)
// handle err
```

### Deleting Aggregates

```go
type example struct { ... }

repo := aggregate.NewRepository(...)
a, err := repo.Fetch(context.TODO(), "foo", uuid.New())
// handle err

err = repo.Delete(context.TODO(), a)
// handle err
```

### Querying Aggregates

```go
repo := aggregate.NewRepository(...)
cur, err := repo.Query(context.TODO(), query.New(
  query.Name("foo", "bar", "baz"), // query by aggregate names
  query.ID(uuid.New(), uuid.New(), uuid.New()), // query by aggregate ids
  query.Version( // query by aggregate versions
    version.Exact(4, 7, 10), 
    version.InRange(version.Range{5, 10}),
    version.Min(2),
    version.Max(20),
  ),
))
// handle err

result, err := cursor.All(context.TODO(), cur)
// handle err

log.Println(result)
```

### Aggregates from Event Stream

```go
var events event.Cursor

cur := stream.New(
  context.TODO(),
  events,

  // provide the factory func for the "foo aggregate
  stream.AggregateFactory("foo", func(id uuid.UUID) aggregate.Aggregate {
    return NewExample(id)
  }),

  // give hint that events are sorted
  stream.Sorted(),
)

for cur.Next(context.TODO()) {
  a := cur.Aggregate()
  log.Println(a)
}

if err := cur.Err(); err != nil {
  // handle err
}
```

### Requirements

- save aggregates
- fetch aggregates
- validate aggregate integrity (optimistic concurrency)
