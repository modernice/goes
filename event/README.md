# Events

Package `event` is the core of goes. It defines and implements a generic event
system that is used as the building block for all the other components
provided by goes.

The core type of this package is the `Event` interface. An event is either an
application event or an aggregate event, depending on the provided data.
Read the documentation of the `Event` interface for more information.

To create an event, pass at least the name of the event and some arbitrary
event data to the `New` function:

```go
import "github.com/modernice/goes/event"

func example() {
	evt := event.New("foo", 3)
	// evt.ID() == uuid.UUID{...}
	// evt.Name() == "foo"
	// evt.Time() == time.Now()
	// evt.Data() == 3
}
```

Events can be published and subscribed to over an event bus:

```go
import "github.com/modernice/goes/event"

func example(bus event.Bus) {
	evt := event.New("foo", 3)
	err := bus.Publish(context.TODO(), evt)
}
```

Events can also be subscribed to using an event bus:

```go
import "github.com/modernice/goes/event"

func example(bus event.Bus) {
	// Subscribe to "foo", "bar", and "baz" events.
	events, errs, err := bus.Subscribe(context.TODO(), "foo", "bar", "baz")
```

Events can be inserted into and queried from an event store:

```go
import "github.com/modernice/goes/event"

func example(store event.Store) {
	evt := event.New("foo", 3)
	err := store.Insert(context.TODO(), evt)

	events, errs, err := store.Query(context.TODO(), query.New(
		query.Name("foo"), // query "foo" events
		query.SortByTime(), // sort events by time
 ))
```

Depending on the used event store and/or event bus implementations, it may be
required to pass an Encoding for your event data to the store/bus. Example
using encoding/gob for encoding of event data:

```go
import (
	"github.com/modernice/goes/backend/mongo"
	"github.com/modernice/goes/event"
)

func example() {
	enc := codec.Gob(event.NewRegistry())
	codec.GobRegister[int](enc, "foo") // register "foo" as an int
	codec.GobRegister[string](enc, "bar") // register "bar" as a string
	codec.GobRegister[struct{Foo string}](enc, "baz") // register "baz" as a struct{Foo string}

	store := mongo.NewEventStore(enc)
```
