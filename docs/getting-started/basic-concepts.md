<script setup>
import Card from '../components/Card.vue'
</script>

# Basic Concepts

goes consits of multiple components that build on top of each other. The core
components are [**Events**](#event-system), [**Aggregates**](#aggregates) and
[**Projections**](#projections).

## Event System

goes core is the **Event System**. The Event System defines the interface for
the `Event`, `Event Bus` and `Event Store` types. It also provides a set of
in-memory implementations for these interfaces, which are useful for testing and
prototyping. However, you will probably want to use production-ready
implementations such as the [MongoDB Event Store](/integrations/event-store/mongodb)
and the [NATS Event Bus](/integrations/event-bus/nats).

### Events

goes defines the `Event` interface, which represents an event that has happened
in your application. An event typically represents a Domain Event, that is a
state change in one of your [aggregates](/core/aggregates/) but it can also be
used an [**Integration Event**](https://codeopinion.com/should-you-publish-domain-events-or-integration-events/).

#### Type Definition

```go
// github.com/modernice/goes/event
package event

type Event = Of[any]

type Of[Data any] interface {
	ID() uuid.UUID
	Name() string
	Time() time.Time
	Data() Data
	Aggregate() (id uuid.UUID, name string, version int)
}
```

The difference between these two types of events is that a Domain Event is bound
to a specific aggregate's event stream, while an Integration Event exists outside
of the [domain's boundaries](https://learn.microsoft.com/en-us/dotnet/architecture/microservices/architect-microservice-container-applications/identify-microservice-domain-model-boundaries)
to facilitate communication between different services. In goes, the "type" of
an event is determined by its aggregate metadata.


### Creating Events

To create an event, you can use the `event.New` function, which returns an
`event.Evt` struct that implements the `Event` interface. To create an event,
you need to provide at least the **event name** and the **event data**.

For example, if you want to create an event that represents a user registration,
you could do the following:

```go
package main

import (
	"github.com/modernice/goes/event"
)

func main() {
	evt := event.New("user.registered", "bob@example.com")

	// evt.ID() == uuid.UUID{...} // auto-generated
	// evt.Name() == "user.registered"
	// evt.Data() == "bob@example.com"
	// evt.Time() == time.Now() // roughly
	// evt.Aggregate() == (uuid.Nil, "", 0)
}
```

### Aggregate Events

To bind an event to a specific aggregate's event stream, you can apply the
`event.Aggregate` option to specify the event's position (version) in the aggregate's
event stream.

```go
package main

import (
	"github.com/google/uuid"
	"github.com/modernice/goes/event"
)

func main() {
	id := uuid.New()
	name := "user"
	version := 1

	evt := event.New(
		"user.registered",
		"bob@example.com",
		event.Aggregate(id, name, version),
	)

	// evt.ID() == uuid.UUID{...} // auto-generated
	// evt.Name() == "user.registered"
	// evt.Data() == "bob@example.com"
	// evt.Time() == time.Now() // roughly
	// evt.Aggregate() == (id, name, version)
}
```

The **version** is the event's
[Optimistic Concurrency Control](https://teivah.medium.com/event-sourcing-and-concurrent-updates-32354ec26a4c)
version in the aggregate's event stream.

### Event Bus

The Event Bus allows you to publish events and subscribe to events from
different, potentially distributed parts or services of your application.
This allows you to decouple your application's components and services,
by allowing them to asynchronously communicate through events.

#### Type Definition

The `event.Bus` interface combines the `event.Publisher` and `event.Subscriber` interfaces:

```go
// github.com/modernice/goes/event
package event

type Bus interface {
	Publisher
	Subscriber
}

type Publisher interface {
	Publish(ctx context.Context, events ...Event) error
}

type Subscriber interface {
	Subscribe(ctx context.Context, names ...string) (<-chan Event, <-chan error, error)
}
```

#### Subscribing to Events

To subscribe to events, you can use the `Bus.Subscribe` method, which accepts a
Context and a variadic list of event names. The subscription will be canceled
when the provided Context is canceled.

```go
package main

import (
	"context"

	"github.com/modernice/goes/event/eventbus"
)

var bus = eventbus.New()

func main() {
	events, errs, err := bus.Subscribe(context.TODO(), "foo", "bar", "baz")
	if err != nil {
		panic(fmt.Errorf("failed to subscribe: %w", err))
	}

	// events == <-chan event.Event
	// errs == <-chan error
}
```

In the example above, we subscribe to the events "foo", "bar" and "baz".
The `Subscribe` method returns two channels: one for the events and one for the
subscription errors. The `Subscribe` method also returns an error, which is
non-nil if the subscription failed.

#### Iterating Event Subscriptions

To make it easier to work with streams (channels), goes provides the [Streams](/helpers/streams)
helper package that provides convenient helpers for working with these channels.

For example, to iterate over the streams until an error occurs, you can use the
[streams.Walk](/helpers/streams#walk) function:

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventbus"
	"github.com/modernice/goes/helper/streams"
)

var bus = eventbus.New()

func main() {
	events, errs, err := bus.Subscribe(context.TODO(), "foo", "bar", "baz")
	if err != nil {
		panic(fmt.Errorf("failed to subscribe: %w", err))
	}

	err = streams.Walk(context.TODO(), func(evt event.Event) error {
		log.Println(evt.Name())
		return nil
	}, events, errs)

	if err != nil {
		panic(fmt.Errorf("subscription error: %w", err))
	}
}
```

#### Publishing Events

To-Do

### Event Store

To-Do

#### Type Definition

To-Do

#### Inserting Events

To-Do

#### Querying Events

To-Do

## Aggregates

To-Do

## Projections

To-Do
