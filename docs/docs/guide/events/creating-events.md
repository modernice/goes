# Creating Events

An event is any type that implements the
[event.Event / event.Of[T]](/guide/events/#event) interface. You can create such
an event with the `event.New()` function.

The following example creates a `"user.created"` event with the string `"Bob"` as its event data.

```go
package example

import (
  "time"

  "github.com/modernice/goes/event"
  "github.com/modernice/goes/internal"
)

func example() {
  evt := event.New("user.created", "Bob")

  evt.ID() == internal.NewUUID() // randomly generated UUIDv7
  evt.Name() == "user.created"
  evt.Data() == "Bob"
  evt.Time() == time.Now() // roughly
}
```

## Event Data

You can use any type as event data. In the previous example, we created an event
with a `string` as its event data, representing the name of the user. Now, when
registering a user, you typically need to put more information in the event data,
like the user's email address and password. You can use structs as event data,
like so:

```go
package myapp

// UserRegisteredData is the event data for the "user.registered" event.
type UserRegisteredData struct {
  Name string
  Email string
  Password string
}
```

Then create the event with that event data:

```go
package myapp

import "github.com/modernice/goes/event"

func example() {
  evt := event.New("user.registered", UserRegisteredData{
    Name: "Bob",
    Email: "bob@example.com",
    Password: "<some-encrypted-password>",
  })

  evt.Data() == UserRegisteredData{
    Name: "Bob",
    Email: "bob@example.com",
    Password: "<some-encrypted-password>",
  }
}
```

## Event Options

The `event.New()` function accepts functional options to override the default
`ID` and `Time` of the event.

```go
package example

import (
  "time"
  "github.com/modernice/goes/internal"
  "github.com/modernice/goes/event"
)

func example() {
  id := internal.NewUUID()
  t := time.Now().Add(-time.Hour) // 1 hour ago

  evt := event.New(
    "user.registered", 
    "Bob",
    event.ID(id),
    event.Time(t),
  )

  evt.ID() == id
  evt.Time() == t
}
```

## Aggregate Events

::: tip
Typically, you will create aggregate events using `aggregate.Next()` instead
of `event.New()`. Read the [Aggregate](/guide/aggregates/) documentation for
more information.
:::

An **Aggregate Event** is an event that additionally provides its position in
the event stream of an aggregate. To create an such an event, you can pass the
`event.Aggregate()` option to `event.New()`. The option accepts the id and name
of the aggregate as well as the position in the event stream, starting at
position `1`.

::: info
An event's position in the event stream of an aggregate is denoted by the **Aggregate Version** of the event.
:::

```go
package example

import "github.com/modernice/goes/event"

func example() {
  id := internal.NewUUID()
  name := "auth.user"
  version := 1 // position in the event stream

  evt := event.New(
    "user.registered",
    "Bob",
    event.Aggregate(id, name, version),
  )

  aid, aname, aversion := evt.Aggregate()

  aid == id
  aname == name
  aversion == version
}
```

## Custom Implementation

`event.New()` returns an `event.Evt[T]`, which implements the
`event.Of[T] / event.Event` interface. If you don't want to depend on the
implemenation provided by goes, you can create a custom implementation:

```go
package myapp

type UserRegistered struct {
  Name string
  Email string
  Password string

  userID uuid.UUID

  evtID uuid.UUID
  evtTime time.Time
  evtVersion int
}

func (evt UserRegistered) ID() uuid.UUID {
  return evt.evtID
}

func (evt UserRegistered) Time() time.Time {
  return evt.evtTime
}

func (evt UserRegistered) Data() UserRegistered {
  return evt
}

func (evt UserRegistered) Aggregate() (uuid.UUID, string, int) {
  return userID, "auth.user", evt.evtVersion
}
```

