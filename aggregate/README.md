# Aggregates

The `aggregate` package builds on top of the [Event System](../event) to provide
event-sourced aggregate tooling.

## Design

The `aggregate.Aggregate` interface defines the minimum method set of an
aggregate. The `*aggregate.Base` type implement this interface and can be
embedded into your structs to provide the base implementation for your
aggregates. The `aggregate.New()` function instantiates the `*aggregate.Base`
type.

```go
package aggregate

type Aggregate interface {
	// Aggregate returns the id, name and version of the aggregate.
	Aggregate() (uuid.UUID, string, int)

	// AggregateChanges returns the uncommited events of the aggregate.
	AggregateChanges() []event.Event

	// ApplyEvent applies the event on the aggregate.
	ApplyEvent(event.Event)
}
```

```go
// Package auth is an example authentication service.
package auth

const UserAggregate = "auth.user"

type User struct {
  *aggregate.Base
}

// NewUser returns the user with the given id.
func NewUser(id uuid.UUID) *User {
  return &User{
    Base: aggregate.New(UserAggregate, id),
  }
}
```

### Example

Example user aggregate:

```go
package auth

import (
  "github.com/modernice/goes/event"
  "github.com/modernice/goes/aggregate"
)

// UserAggregate is the name of the User aggregate.
const UserAggregate = "auth.user"

// Events
const (
  UserRegistered = "auth.user.registered"
)

// UserRegisteredData is the event data for UserRegistered.
type UserRegisteredData struct {
  Name  string
  Email string
}

// User represents a user of the application.
type User struct {
  *aggregate.Base

  Name  string
  Email string
}

// NewUser returns the user with the given id.
func NewUser(id uuid.UUID) *User {
  return &User{
    Base: aggregate.New(UserAggregate, id),
  }
}

// Register registers the user with the given name and email address.
func (u *User) Register(name, email string) error {
  if name = strings.TrimSpace(name); name == "" {
    return errors.New("empty name")
  }

  if err := validateEmail(email); err != nil {
    return errors.New("invalid email %q: %v", email, err)
  }

  // aggregate.NextEvent() creates and applies the next event for the User using
  // u.ApplyEvent(evt). u.ApplyEvent then calls u.register(evt) which actually
  // updates the state of the User.
  aggregate.NextEvent(u, UserRegistered, UserRegisteredData{
    Name:  name,
    Email: email,
  })

  return nil
}

func (u *User) register(evt event.Event) {
  data := evt.Data().(UserRegisteredData)
  u.Name = data.Name
  u.Email = data.Email
}

// ApplyEvent overrides the ApplyEvent function of u.Base.
func (u *User) ApplyEvent(evt event.Event) {
  switch evt.Name() {
  case UserRegistered:
    u.register(evt)
  }
}
```

## Repository

The [github.com/modernice/goes/aggregate/repository](
../aggregate/repository) package implements the `aggregate.Repository` interface.
The aggregate repository uses the underlying event store to save and
query aggregates from and to the event store.

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
which accepts a [query](../aggregate/query) to filter aggregate events from the
event store.

Queries return streams of `aggregate.History`s, which provide the name and id of
the streamed aggregate. Histories can be applied onto aggregates to build their
current state.

```go
package example

import (
  "context"
  "github.com/modernice/goes/aggregate"
  "github.com/modernice/goes/aggregate/query"
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

- [~~Create & Test an Aggregate~~ (To-Do)](../examples/aggregate)
- [~~Create Projections~~ (To-Do)](../examples/projection)

## Stream Helpers

goes provides helper functions to work with channels of aggregates and aggregate tuples:

- `aggregate.Walk`
- `aggregate.Drain`
- `aggregate.ForEach`
- `aggregate.WalkRefs`
- `aggregate.DrainRefs`
- `aggregate.ForEachRef`

