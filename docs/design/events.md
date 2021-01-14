# Events

The `Event` module provides a distributed event system.

Given an application that uses the Event module, that application can publish
Events through an Event Bus to any number of interested parts in our application
which can then execute actions based on the published Event.

## Event

An `Event` is a fact about something that happened in the domain of a consuming
application and is typically bound to an aggregate. In a hypothetical "blog"
application, when the author publishes a blog post such an Event could be named
`post.published` and besides name, timestamp and aggregate of the event contain
data about the author who published it.

Event data must be a struct containing any number of properties and must be
provided by the consuming application. An empty struct must be the smallest
possible event data.

An Event may be bound to an aggregate. This means that it's raised/created by an
event-sourced [Aggregate](./aggregates.md) in the application. Event-sourced
aggregates exist in the database only as a series of Events and are constructed
by fetching those Events from the database and applying them one by one in order
on a freshly instantiated Aggregate of specified name/type.

The Aggregate version field of an Event is needed to prevent Optimistic
Concurrency errors cannot persist and violate the integrity of an
[Aggregate](./aggregates.md).

### Example

```go
type exampleEvent struct {
  FieldA string
}

evt := event.New(
  "foo", // name of the event
  exampleEvent{FieldA: "bar"}, // event data
)

log.Println(fmt.Sprintf("event name: %s", evt.Name()))
log.Println(fmt.Sprintf("event id: %s", evt.ID()))
log.Println(fmt.Sprintf("aggregate name: %s", evt.AggregateName()))
log.Println(fmt.Sprintf("aggregate id: %s", evt.AggregateID()))
log.Println(fmt.Sprintf("timestamp: %s", evt.Time()))
log.Println(fmt.Sprintf("event data: %#v", evt.Data()))
```

### Requirements

Events must have the following properties:

- unique id
- event name
- event data
- timestamp
- aggregate name (may be empty)
- aggregate id (may be empty)
- aggregate version

## Event Bus

In order to listen for Events and subscribe to Events module must provide an
`Event Bus`. An Event Bus accepts an Event and passes is to every handler that
subscribed to the given Event name.

The Event Bust must be able to pass Events across processes. In order for that
to work, internally the Event Bus should use an existing streaming solution like
one of the following:

- [Apache Kafka](https://github.com/apache/kafka)
- [NATS](https://github.com/nats-io/nats-server)

While Kafka is a more mature streaming solution, NATS is more lightweight and
less resource heavy (NATS is written in Go, Kafka in Java). Besides that, NATS
doesn't provide persistance for published events, but persistance is the
responsibility of the [Event Store](#event-store) anyway.

### Example

```go
type exampleEvent struct {
  FieldA string
}

bus := nats.New()

// Publish an "example" Event
err := bus.Publish(context.TODO(), event.New("example", exampleEvent{
  FieldA: "foo",
}))
// handle err

// Subscribe to "example" Events
events, err := bus.Subscribe(context.TODO(), "example")
// handle err

// events is a channel of Events
for evt := range events {
  log.Println(fmt.Sprintf("%#v", evt))
}
```

### Requirements

- publish events
- subscribe to events

## Event Store

Published Events need to be persisted in a database for later retrieval. This is
especially important for Aggregate Events which are needed to reconstruct the
state of the Aggregate they belong to (because Aggregates exist in the database
only as a series of Events). Said persistance is handled by an `Event Store`.

The underlying database for the Event Store is not really important as long as
it supports indexing, so we will mainly stick with **MongoDB**, but we must make
this modular so the database can be swapped with another one.

### Example

```go
store, err := mongostore.New(...)
// handle err

// Insert an Event
err := store.Insert(context.TODO(), event.New(...))
// handle err
```

**Example with Event Bus:**
```go
store, err := mongostore.New(...)
// handle err

bus := eventbus.WithStore(nats.New(), store)

// Publish and store Event
err := bus.Publish(context.TODO(), event.New(...))
// handle err
```

### Requirements

- insert events
- query events by
  - event name
  - event id
  - aggregate name
  - aggregate id
  - aggregate version
  - timestamp
- delete events
