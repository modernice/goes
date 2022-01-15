# Aggregates

The `aggregate` package provides the tools to create and manage event-sourced aggregates.

## Design

The `aggregate.Aggregate` interface defines the minimum method set of an
aggregate that is usable by goes. This interface is implemented by the
`aggregate.Base` type that can be embedded into your structs.

```go
package aggregate

type Aggregate interface {
	AggregateID() uuid.UUID
	AggregateName() string
	AggregateVersion() int
	AggregateChanges() []event.Event
	TrackChange(...event.Event)
	FlushChanges()
	ApplyEvent(event.Event)
	SetVersion(int)
}
```

### Base Aggregate

The `*aggregate.Base` type can be embedded into structs to make them implement
the `aggregate.Aggregate` interface. The `aggregate.New` function instantiates
the `*aggregate.Base` type.

```go
// Package auth is an example authentication service.
package auth

const UserAggregate = "auth.user"

type User struct {
  *aggregate.Base
}

func NewUser(id uuid.UUID) *User {
  return &User{
    Base: aggregate.New(UserAggregate, id),
  }
}
```

## Repository

The [github.com/modernice/goes/tree/main/aggregate/repository](
http://github.com/modernice/goes/tree/main/aggregate/repository) package implements the
`aggregate.Repository` interface, which provides querying of aggregates through
the event store.

```go
package aggregate

type Repository interface {
	Save(ctx context.Context, a Aggregate) error
	Fetch(ctx context.Context, a Aggregate) error
	FetchVersion(ctx context.Context, a Aggregate, v int) error
	Query(ctx context.Context, q Query) (<-chan History, <-chan error, error)
	Delete(ctx context.Context, a Aggregate) error
}
```

### Fetch an Aggregate

To fetch the current state of an aggregate, you need to pass the already
instantiated aggregate to the `aggregate.Repository.Fetch` method.

```go
package example

func fetchAggregate(repo aggregate.Repository) {
  userID := uuid.New() // Get this from somewhere
  u := NewUser(userID) // Instantiate the aggregate
  
  err := repo.Fetch(context.TODO(), u) // Fetch and apply events
  // handle err
}
```

### Query Aggregates

You can query multiple aggregates with the `aggregate.Repository.Query` method,
which accepts a [query](http://github.com/modernice/goes/tree/main/aggregate/query) to
filter aggregate events from the event store.

Queries return streams of `aggregate.History`s, which statically provide the
name and id of the streamed aggregate. Histories can be applied onto aggregates
to build their current state.

```go
package example

import (
  "context"
  "github.com/modernice/goes/tree/main/aggregate"
  "github.com/modernice/goes/tree/main/aggregate/query"
)

func queryAggregates(repo aggregate.Repository) {
  str, errs, err := repo.Query(context.TODO(), query.New(
    query.Name("auth.user"), // Query "auth.user" aggregates
  ))
  // handle err

  histories, err := aggregate.Drain(context.TODO(), str, errs) // Drain the stream
  // handle err

  for _, h := range histories {
    u := NewUser(h.AggregateID()) // Instantiate the aggregate
    h.Apply(u) // Build the aggregate state
  }
}
```

### Delete an Aggregate

Deleting an aggregate means deleting all its events from the event store.

```go
package example

func deleteAggregate(repo aggregate.Repository) {
  userID := uuid.New() // Get this from somewhere
  u := NewUser(userID)

  err := repo.Delete(context.TODO(), u)
  // handle err
}
```

## Guides

- [Create & Test an Aggregate](http://github.com/modernice/goes/tree/main/examples/aggregate)
- [Create Projections](http://github.com/modernice/goes/tree/main/examples/projection)

## Stream Helpers

goes provides helper functions to work with channels of aggregates and aggregate tuples:

- `aggregate.Walk`
- `aggregate.Drain`
- `aggregate.ForEach`
- `aggregate.WalkRefs`
- `aggregate.DrainRefs`
- `aggregate.ForEachRef`

