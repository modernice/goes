# Codec Registry

The codec registry maps event and command names to Go types. It handles serialization and deserialization — when the event store or event bus needs to encode or decode event data, the codec knows which Go type corresponds to which name.

> For a step-by-step introduction, see the [Tutorial](/tutorial/04-codec-registry).

## Creating a Registry

```go
import "github.com/modernice/goes/codec"

reg := codec.New()
```

By default, the registry uses `encoding/json` for marshaling and unmarshaling.

## Registering Types

Use `codec.Register` to map an event name to a Go type:

```go
codec.Register[ProductCreatedData](reg, ProductCreated)
codec.Register[PriceChangedData](reg, PriceChanged)
codec.Register[StockAdjustedData](reg, StockAdjusted)
```

The convention from the tutorial is to group registrations into functions per aggregate:

```go
func RegisterProductEvents(r codec.Registerer) {
	codec.Register[ProductCreatedData](r, ProductCreated)
	codec.Register[PriceChangedData](r, PriceChanged)
	codec.Register[StockAdjustedData](r, StockAdjusted)
}
```

Command payloads are registered the same way — the codec handles both events and commands.

## Marshal and Unmarshal

The registry implements `codec.Encoding`:

```go
type Encoding interface {
	Marshal(any) ([]byte, error)
	Unmarshal([]byte, string) (any, error)
}
```

`Marshal(data)` encodes data to bytes. `Unmarshal(bytes, name)` uses the name to look up the registered type, creates a new instance, and decodes into it.

You rarely call these directly — the event store, event bus, and command bus use the codec internally.

## Custom Marshalers

To override the default JSON encoding for a specific type, implement `codec.Marshaler` or `codec.Unmarshaler` on your data type:

```go
type Marshaler interface {
	Marshal() ([]byte, error)
}

type Unmarshaler interface {
	Unmarshal([]byte) error
}
```

```go
// Marshal uses a value receiver — event data is passed by value to the codec.
func (d ProductCreatedData) Marshal() ([]byte, error) {
	return msgpack.Marshal(d)
}

// Unmarshal uses a pointer receiver — the codec creates a pointer to the
// registered type and decodes into it.
func (d *ProductCreatedData) Unmarshal(b []byte) error {
	return msgpack.Unmarshal(b, d)
}
```

`Marshal` must use a **value receiver** because the codec receives event data by value (from `evt.Data()`). A pointer-receiver method would not be found. `Unmarshal` must use a **pointer receiver** because the codec creates a pointer to the registered type internally and needs to decode into it.

## Changing the Default

Replace the default JSON marshaler for all types. For example, using [MessagePack](https://msgpack.org/):

```go
reg := codec.New(
	codec.Default(msgpack.Marshal, msgpack.Unmarshal),
)
```

The `codec.Default` option accepts a marshal function `func(any) ([]byte, error)` and an unmarshal function `func([]byte, any) error`. This changes the encoding for all registered types that don't implement `codec.Marshaler`/`codec.Unmarshaler` themselves.

## Debug Mode

```go
reg := codec.New(codec.Debug(true))
```

Enables debug logging for type registration and encoding operations.
