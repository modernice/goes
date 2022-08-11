# Publish & Subscribe

You can use an event bus to asynchronously communicate between different
services, and between different parts of your application. When publishing an
event over an event bus, subscribers within the same event bus will receive that 
event.

## Example

When subscribing to events, you need to pass the event names to subscribe to.
The event bus will return an event stream and an error stream for the
subscription. A subscription will be cancelled when the provided `Context` is
cancelled.

::: tip
For demonstration purposes, this example makes use of the
[In-Memory](/guide/backends/event-bus/in-memory) event bus. When using for
example the [NATS](/guide/backends/event-bus/nats) event bus, publishing and
subscribing can be done within different services that run in different processes.
:::

```go
package example

import (
  "context"
  "fmt"
  "log"
  "github.com/modernice/goes/event"
  "github.com/modernice/goes/event/eventbus"
  "github.com/modernice/goes/helper/streams"
)

func example() {
  // Create an in-memory event bus.
  bus := eventbus.New()

  go subscribe(bus)
  publish(bus)
}

func subscribe(bus event.Bus) {
  // Subscribe to "foo" and "bar" events.
  events, errs, err := bus.Subscribe(context.TODO(), "foo", "bar")
  if err != nil {
    panic(fmt.Errorf(
      "subscribe to %v events: %w",
      []string{"foo", "bar"},
      err,
    ))
  }

  // Walk the event stream and log each incoming event.
  if err := streams.Walk(
    context.TODO(),
    func(evt event.Event) error {
      log.Printf(
        "received %q event with event data: %v",
        evt.Name(),
        evt.Data(),
      )
    },
    events,
    errs,
  ); err != nil {
    panic(fmt.Errorf("walk stream: %w", err))
  }
}

func publish(bus event.Bus) {
  events := []event.Event{
    event.New("foo", "foo-data").Any(),
    event.New("bar", "bar-data").Any(),
  }

  if err := bus.Publish(context.TODO(), events...); err != nil {
    panic(fmt.Errorf("publish events: %w", err))
  }
}
```

As you might noticed, in the above `publish` function, when creating the events,
their `Any()` methods are called. This is necessary in this case because of Go's
current limitations in type parameters (generics). The `Publish()` method of the
`event.Bus` interface expects the `event.Event / event.Of[any]` type, but when
creating an event with for example a `string` as event data, the created type
will be `event.Of[string]`. The `Any()` method of an event returns the event as
an `event.Event / event.Of[any]`.
