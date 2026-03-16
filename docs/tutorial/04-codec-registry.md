# 4. Codec Registry

When events are stored in a database (MongoDB, PostgreSQL), the framework needs to know how to convert your event data types to and from bytes. The codec registry maps event names to their Go types.

## Register Events

Add a `RegisterProductEvents` function to `product.go`:

```go
import "github.com/modernice/goes/codec"

func RegisterProductEvents(r codec.Registerer) {
	codec.Register[ProductCreatedData](r, ProductCreated)
	codec.Register[string](r, ProductRenamed)
	codec.Register[int](r, PriceChanged)
	codec.Register[StockAdjustedData](r, StockAdjusted)
}
```

## Wire It Up

In `cmd/main.go`, call the registration function:

```go
import (
	// ... existing imports ...
	"github.com/yourname/shop"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	eventReg := codec.New()

	// Register all event types.
	shop.RegisterProductEvents(eventReg) // [!code ++]

	store := eventstore.New()
	bus := eventbus.New()
	store = eventstore.WithBus(store, bus)

	// ...
}
```

## How It Works

`codec.Register[T](registry, eventName)` tells the registry:

> "When you see an event named `eventName`, the data is of type `T`. Use JSON to serialize and deserialize it."

The generic type parameter `T` must match the type you use in your event data:

```go
// These must match:
codec.Register[ProductCreatedData](r, ProductCreated)
//             ^^^^^^^^^^^^^^^^^^
aggregate.Next(p, ProductCreated, ProductCreatedData{...})
//                                ^^^^^^^^^^^^^^^^^^
event.ApplyWith(p, p.created, ProductCreated)
// handler signature: func(evt event.Of[ProductCreatedData])
//                                      ^^^^^^^^^^^^^^^^^^
```

## Default Serialization

The codec uses JSON by default. Your event data types just need to be JSON-serializable — exported fields with standard Go types. If you need a different format, you can configure it:

```go
eventReg := codec.New(codec.Default(
	customMarshal,   // func(any) ([]byte, error)
	customUnmarshal, // func([]byte, any) error
))
```

Or implement `codec.Marshaler` / `codec.Unmarshaler` on individual types for per-type customization.

## Convention

We recommend adding a `Register<Aggregate>Events` function to each aggregate file. As we add more aggregates, `cmd/main.go` will call each one:

```go
shop.RegisterProductEvents(eventReg)
shop.RegisterOrderEvents(eventReg)    // coming in chapter 7
shop.RegisterCustomerEvents(eventReg) // coming in chapter 8
```

## Next

Events are registered. Now let's [persist and fetch our aggregates](./05-repository) using repositories.
