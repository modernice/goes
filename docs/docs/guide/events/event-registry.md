# Event Registry

Depending on the used `event.Bus` and `event.Store` implementations, it may be
necessary to create a registry for the events of your application or service.

## Why?

When you publish an event over an event bus (other than the in-memory bus), or
insert an event into an event store (other than the in-memory store), the bus or
store needs to somehow encode the event data so that it can be transferred over
the network. An event registry allows you to configure how event data should be
encoded and decoded.

## Example

A registry can be created using `event.NewRegistry()`, which is just an alias
for `codec.New()`. By default, the registry encodes and decodes event data using
the standard `encoding/json` package.

```go
package example

import (
  "github.com/modernice/goes/codec"
  "github.com/modernice/goes/event"
)

// FooData is the event data for the "foo" event.
type FooData struct {
  Foo string
  Bar int
}

func example() {
  r := event.NewRegistry()
  r := codec.New() // also works

  // Register the FooData type for the "foo" event.
  codec.Register[FooData](r, "foo")

  // Register the "bar" event as an int.
  codec.Register[int](r, "bar")

  // Register the "baz" event as a string.
  codec.Register[string](r, "baz")

  // Encode event data.
  bytes, err := r.Marshal(FooData{Foo: "foo", Bar: 3})
  if err != nil {
    panic(fmt.Errorf("encode event data: %w", err))
  }

  // Decode event data.
  decoded, err := r.Unmarshal(bytes, "foo")
  if err != nil {
    panic(fmt.Errorf("decode %q event data: %w", "foo", err))
  }

  // decoded.(FooData) == FooData{Foo: "foo", Bar: 3}
}
```

## Default Encoding

By default, event data is encoded using `encoding/json`. You can override the
default encoding with the `codec.Default()` option by providing the default
marshal and unmarshal functions. The following example uses `encoding/gob` to
encode and decode event data.

```go
package example

import (
  "encoding/gob"
  "github.com/modernice/goes/codec"
  "github.com/modernice/goes/event"
)

func example() {
  r := event.NewRegistry(codec.Default(
    func(data any) ([]byte, error) {
      var buf bytes.Buffer
      err := gob.NewEncoder(&buf).Encode(data)
      return buf.Bytes(), err
    },

    func(b []byte, data any) error {
      return gob.NewDecoder(bytes.NewReader(b)).Decode(data)
    },
  ))
}
```

## Custom Marshaler

Encoding and decoding of event data can also be implemented by the event data
itself. When an event data type implements the `codec.Marshaler` and/or
`codec.Unmarshaler` interface, the registry will use the event data's custom
marshal functions instead of the configured defaults.

```go
package example

import (
  "github.com/modernice/goes/codec"
  "github.com/modernice/goes/event"
)

// FooData is the event data for the "foo" event.
type FooData struct {
  Foo string
  Bar int
}

func (data FooData) Marshal() ([]byte, error) {
  return []byte(fmt.Sprintf("%s:%d", data.Foo, data.Bar)), nil
}

func (data *FooData) Unmarshal(b []byte) (err error) {
  parts := strings.Split(string(b), ":")
  if len(parts) != 2 {
    return fmt.Errorf("invalid event data")
  }
  data.Foo = parts[0]
  data.Bar, err = strconv.Atoi(parts[1])
  return err
}

func example() {
  r := event.NewRegistry()
  r := codec.New() // also works

  // Register the FooData type for the "foo" event.
  codec.Register[FooData](r, "foo")

  // Encode event data.
  bytes, err := r.Marshal(FooData{Foo: "foo", Bar: 3})
  if err != nil {
    panic(fmt.Errorf("encode event data: %w", err))
  }

  // Decode event data.
  decoded, err := r.Unmarshal(bytes, "foo")
  if err != nil {
    panic(fmt.Errorf("decode %q event data: %w", "foo", err))
  }

  // decoded.(FooData) == FooData{Foo: "foo", Bar: 3}
}
```

## Utilities

### Register event data

Registers the event data type for a specific event name.

```go
package example

type FooData struct { ... }

func example() {
  r := event.NewRegistry()

  codec.Register[FooData](r, "foo")
}
```

### Instantiate event data by event name

The `codec.Make()` function creates an empty instance of event data for a
specific event name.

::: warning
If the provided type parameter does not match the actual type of the created
event data, an error is returned.
:::

```go
package example

type FooData struct { ... }
  
func example(r *codec.Registry) {
  data, err := codec.Make[FooData](r, "foo")
  if err != nil {
    panic(fmt.Errorf("create %q event data: %w", "foo", err))
  }

  data == FooData{}
}
```
