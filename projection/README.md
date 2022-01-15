# Projections

The `projection` package provides the boilerplate to create and apply
projections from [events](../event).


## Projections

### Event Applier

The basic interface that a projection type needs to implement is the
`projection.EventApplier` interface, which defines a single
`ApplyEvent(event.Event)` method. A projection only needs to be able to apply
events onto itself. Additional behavior can be added using [extensions](
#extensions).

```go
package projection

// An EventApplier applies events onto itself to build the projection state.
type EventApplier interface {
	ApplyEvent(event.Event)
}
```

### Apply Events

Given a projection type that has an `ApplyEvent(event.Event)` method, you can
use the `projection.Apply()` method to apply a given set of events onto the
projection. Given a basic projection `p` that does not implement any
[extensions](#extensions), this does the same as iterating over the given events
and manually calling `p.ApplyEvent(evt)`.

```go
package example

import (
  "log"
  "github.com/modernice/goes/event"
  "github.com/modernice/goes/projection"
)

type Foo struct {}

func (f *Foo) ApplyEvent(evt event.Event) {
  log.Printf("Event applied: %v", evt.Name())
}

func example() {
  var events []event.Event // e.g. fetched from event store
  var foo Foo // Instantiate your projection type

  err := projection.Apply(&foo, events)
  // handle err
}
```

### Extensions

A projection type may implement additional interfaces that extend or modify the
projection behavior of a projection. Read the documentation of these interfaces
for more information on how to use them:

- `projection.Progressing`
- `projection.Resetter`
- `projection.Guard`
- `projection.HistoryDependent`

## Scheduling

goes' projection tools provide two types of projection schedulers:

- Continuous scheduler
- Periodic scheduler

Schedulers trigger projection jobs which provide a convenient way to efficiently
fetch event from the event store to apply them onto as many projections as needed.


### Continuous Schedule

The continuous schedule subscribes to a given set of events and triggers a
projection job every time one of those events is published over the underlying
event bus.

```go
package example

import (
  "github.com/modernice/goes/event"
  "github.com/modernice/goes/projection/schedule"
)

type Foo struct {}

func (f *Foo) ApplyEvent(evt event.Event) { ... }

func example(bus event.Bus, store event.Store) {
  eventNames := []string{"example.foo", "example.bar", "example.foobar"}
  s := schedule.Continuously(bus, store, eventNames)

  var foo Foo

  errs, err := s.Subscribe(context.TODO(), func(ctx projection.Job) error {
    // Every time the schedule triggers a job, this function is called.
    // The provided projection.Job provides an `Apply` method, which applies
    // the published events onto the passed projection `&foo`.
    return ctx.Apply(ctx, &foo)
  })
  if err != nil {
    log.Fatalf("subscribe to projection schedule: %w", err)
  }

  for err := range errs {
    log.Printf("projection: %v", err)
  }
}
```

The example above subscribes to events with the names "example.foo",
"example.bar" and "example.foobar". Each time such an event is published, the
schedule created a `projection.Job` and calls the provided callback function. In
this example, the callback function just calls the `ctx.Apply()` function, which
itself calls the `projection.Apply()` function with the events from the projection
job.

#### Debounce Jobs

When creating a continuous schedule, the `schedule.Debounce()` option can be
used to debounce the creation of projection jobs when multiple events are
published successively within a short time period. Without the debounce option,
when multiple event are published "at once", one projection job per published
event would be created, which, depending on the complexity within your callback
function, can be a performance hit for your application. The debounce option
ensures that at most one projection job is created within a specified interval.

```go
package example

import (
  "log"
  "github.com/modernice/goes/event"
  "github.com/modernice/goes/projection/schedule"
)

func example(bus event.Bus, store event.Store) {
  eventNames := []string{"example.foo", "example.bar"}
  s := schedule.Continuously(bus, store, eventNames, schedule.Debounce(time.Second))
  s.Subscribe(context.TODO(), func(ctx projection.Job) error {
    log.Println("This function should be called only once.")
    return nil
  })

  // Two events published "at once" but only 1 projection job that provides
  // both events will be created & triggered.
  bus.Publish(context.TODO(), event.New("example.foo", ...))
  bus.Publish(context.TODO(), event.New("example.bar", ...))
}
```

### Periodic Schedule

The periodic schedule does not subscribe to events over an event bus. Instead,
it triggers a projection job in a fixed interval and fetches the entire event
stream of the configured events from the event store within the triggered
projection jobs.

```go
package example

import (
  "github.com/modernice/goes/event"
  "github.com/modernice/goes/projection/schedule"
)

type Foo struct {}

func (f *Foo) ApplyEvent(evt event.Event) { ... }

func example(store event.Store) {
  eventNames := []string{"example.foo", "example.bar", "example.foobar"}
  s := schedule.Continuously(store, eventNames)

  var foo Foo

  errs, err := s.Subscribe(context.TODO(), func(ctx projection.Job) error {
    // ALL "example.foo", "example.bar" and "example.foobar" events are
    // fetched from the event store and applied onto &foo.
    return ctx.Apply(ctx, &foo)
  })
  if err != nil {
    log.Fatalf("subscribe to projection schedule: %w", err)
  }

  for err := range errs {
    log.Printf("projection: %v", err)
  }
}
```

### Projection Jobs

A projection job provides additional query helpers to extract event and
aggregate information from the events in the job. All query functions of the
`projection.Job` use caching to avoid querying the underlying event stream
unnecessarily. Job are thread-safe, which means that they can be applied
concurrently onto multiple projections if needed.

```go
package example

func example(s projection.Schedule) {
  var foo Foo

  errs, err := s.Subscribe(context.TODO(), func(ctx projection.Job) error {
    // Query all events of the job.
    events, errs, err := ctx.Events(ctx)

    // Query all events of the job that belong to one of the given aggregate names.
    events, errs, err := ctx.EventsOf(ctx, "example.foobar")

    // Query all events of the job that would be applied onto the given projection type.
    events, errs, err := ctx.EventsFor(ctx, &foo)

    // Extract all aggregates from the job's events as aggregate.Refs.
    refs, errs, err := ctx.Aggregates(ctx)

    // Extract all aggregates with one of the given names from the job's events
    // as aggregate.Refs.
    refs, errs, err := ctx.Aggregates(ctx, "example.foobar")

    // Extract the first UUID of the aggregate with the given name from the events
    // of the job.
    id, err := ctx.Aggregate(ctx, "example.foobar")

    return nil
  })
  if err != nil {
    log.Fatalf("subscribe to projection schedule: %w", err)
  }

  for err := range errs {
    log.Printf("projection: %w", err)
  }
}
```

#### Trigger Jobs

Both continuous and periodic schedules can be manually triggered at any time
using the `projection.Schedule.Trigger()` method. Manually triggered schedules
always create projection jobs that fetch/query the entire event history of the
configured events when applied onto a projection. This is also true for
continuous schedules, which normally only apply the published events that
triggered the schedule.

```go
package example

func example(s projection.Schedule) {
  errs, err := s.Subscribe(context.TODO(), func(ctx projection.Job) error {
    log.Println("Schedule triggered.")

    // This fetches the entire event history of the events configured
    // in the schedule, even for continuous schedules (but only when
    // triggered manually).
    events, errs, err := ctx.Events(ctx)

    return nil
  })
  // handle err & errs

  err := s.Trigger(context.TODO())
  // handle err
}
```

## Projection Service

The `projection.Service` implements an event-driven projection service, which
allows projection schedules to be triggered from another service/process than it
was defined in.

```go
package service1

func example(reg *codec.Registry, bus event.Bus) {
  // Register the events of the projection service into a registry.
  projection.RegisterService(reg)

  svc := projection.NewService(bus)

  // Given some schedules with names for each of them
  var schedules map[string]projection.Schedule

  // When registering them in the projection service
  for name, s := range schedules {
    svc.Register(name, s)
  }
}

package service2

func example(bus event.Bus) {
  svc := projection.NewService(bus)

  // Another service that uses the same underlying event bus can
  // trigger the registered projection schedules
  err := svc.Trigger(context.TODO(), "foo")
}
```
