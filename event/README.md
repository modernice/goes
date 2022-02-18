# Events

goes defines and implements an event system that can be used both as a simple
generic event system for your application events and also as the underlying
system for event-sourced aggregates. The type that all of goes' components build
around is the `event.Of[any, uuid.UUID]` interface. The `event.E` struct provides the
implementation for `event.Of[any, uuid.UUID]`.

## Design

goes' event design unifies generic events and aggregate events into a single `event.Of[any, uuid.UUID]` type:

```go
package event

type Event interface {
	// ID returns the unique id of the event.
	ID() uuid.UUID
	// Name returns the name of the event.
	Name() string
	// Time returns the time of the event.
	Time() time.Time
	// Data returns the event data.
	Data() any

	// Aggregate returns the id, name and version of the aggregate that the
	// event belongs to. Aggregate should return zero values if the event is not
	// an aggregate event.
	Aggregate() (id uuid.UUID, name string, version int)
}
```

> The `version` field that is returned by `event.Of[any, uuid.UUID].Aggregate` refers to the
optimistic concurrency version of the aggregate the event belongs to. Event
store implementations use this version to do optimistic concurrency checks when
inserting event into the store.

### Event Bus

The `event.Bus[ID] interface defines a simple event bus that is accepted by all
of goes' components. An event bus must be able to publish and subscribe to events:

```go
package event

type Bus interface {
  Publish(context.Context, ...Event) error
  Subscribe(context.Context, ...string) (<-chan Event, <-chan error, error)
}
```

Instead of returning some kind cursor type, the `event.Bus[ID]Subscribe` method
returns two channels: one for the actual subscribed events and one for any
asynchronous errors that happen during the subscription.

It is up to the caller to handle any errors to prevent the application from
blocking. [Common helpers](./#stream-helpers) are provided by goes to avoid
boilerplate code caused by the use of channels.

### Event Store

The `event.Store[ID] interface defines the event store.

```go
package event

type Store interface {
	Insert(context.Context, ...Event) error
	Find(context.Context, uuid.UUID) (Event, error)
	Query(context.Context, Query) (<-chan Event, <-chan error, error)
	Delete(context.Context, ...Event) error
}
```

## Create Events

Events are created using the `event.New` constructor, which returns an `event.E`.
`event.New` requires at least the event name and some user-defined event data.
Additional options may be passed as functional event options.

```go
package example

type FooEventData struct {
  Foo string
}

func createEvent() {
  name := "example.foo"
  data := FooEventData{Foo: "foo"}
  now := time.Now()
  evt := event.New(name, data, event.Time(now))

  evt == event.E{
    Name: "example.foo",
    Data: FooEventData{Foo: "foo"},
    Time: now,
  }
}
```

> If no manual time is provided using the `event.Time()` option, the current time
is used as the event time.

### Create Aggregate Events

The `event.Aggregate` option binds an event to the event stream of an aggregate.

```go
package example

func createAggregateEvent() {
  name := "example.foo"
  data := FooEventData{...}
  aggregateName := "example.foobar"
  aggregateID := uuid.UUID{...}
  aggregateVersion := 3

  evt := event.New(name, data, event.Aggregate(
    aggregateID,
    aggregateName,
    aggregateVersion,
  ))

  id, name, version := evt.Aggregate()
  id == aggregateID
  name == aggregateName
  version == aggregateVersion
}
```

## Event Data

Event data is provided by simple user-defined structs. Depending on the used
event bus and store implementations, events may have to be registered within a
codec so that they can be appropriately encoded and decoded.

```go
package auth

const UserRegistered = "auth.user.registered"

type UserRegisteredData struct {
  Email    string
  Password string
}

func RegisterEvents(r *codec.GobRegistry) {
  r.GobRegister(UserRegistered, func() any { return UserRegisteredData{} })
}
```

## Event Registry

Use `codec.New()` to create a registry for event encoders and decoders. Each
event type registers its own encoder and decoder for its event data:

```go
package example

func setupEvents() {
  reg := codec.New()
  reg.Register(
    "example.foo",
    codec.EncoderFunc(func(w io.Writer, data any) error {
      // encode the event data and write into w
      return nil
    }),
    codec.DecoderFunc(func(r io.Reader) (any, error) {
      // decode and return the event data in r
    }),
  )
}
```

You can then instantiate event data using the name of the event:

```go
package example

func makeEventData(reg *codec.Registry) {
  data, err := reg.New("example.foo")
  // handle err

  data == FooEventData{}
}
```

Or encode and decode event data:

```go
package example

// codec.Encoding is implemented by *codec.Registry
func encodeDecodeEventData(enc codec.Encoding, evt event.Of[any, uuid.UUID]) {
  var buf bytes.Buffer
  err := enc.Encode(&buf, evt.Data())
  // handle err

  data, err := enc.Decode(bytes.NewReader(buf.Bytes()))
  // handle err

  data == evt.Data()
}
```

## Stream Helpers

- `streams.Walk` : Walks a channel of events and calls a function for each event.
	Accepts optional error channels that cause the walk to stop on error.
- `streams.Drain` : Drains a channel of events and returns a slice of all events.
	Accepts optional error channels that cause the drain to stop on error.
- `streams.ForEach` : Walks a channel of events and optional channels of errors
	and calls a function on each event and each error until all channels are closed.
