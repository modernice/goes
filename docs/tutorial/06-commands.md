# 6. Commands

So far, we've been calling aggregate methods directly. Commands add a layer of indirection — instead of calling `product.Create(...)`, you dispatch a `CreateProduct` command. A handler receives the command, loads the aggregate, and executes the business logic.

This matters when you have multiple services, when you want to decouple your API layer from your domain, or when you need to dispatch work asynchronously. Commands also let you coordinate operations across multiple aggregates — for example, placing an order and adjusting product stock in one handler (we'll see this in the [Order chapter](./07-order-aggregate)).

## Define Commands

Add command definitions to `product.go`:

```go
// Command names.
const (
	CreateProductCmd = "shop.product.create"
	RenameProductCmd = "shop.product.rename"
	ChangePriceCmd   = "shop.product.change_price"
	AdjustStockCmd   = "shop.product.adjust_stock"
)

// Command payloads.
type CreateProductPayload struct {
	Name  string
	Price int
	Stock int
}

type AdjustStockPayload struct {
	Quantity int
	Reason   string
}
```

## Register Commands

Add a registration function, just like we did for events:

```go
func RegisterProductCommands(r codec.Registerer) {
	codec.Register[CreateProductPayload](r, CreateProductCmd)
	codec.Register[string](r, RenameProductCmd)
	codec.Register[int](r, ChangePriceCmd)
	codec.Register[AdjustStockPayload](r, AdjustStockCmd)
}
```

## Handle Commands

Add a function that sets up all command handlers for products:

```go
import (
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/command"
	"github.com/modernice/goes/helper/streams"
)

// ProductRepository is the typed repository for products.
type ProductRepository = aggregate.TypedRepository[*Product]

func HandleProductCommands(
	ctx context.Context,
	bus command.Bus,
	products ProductRepository,
) <-chan error {
	createErrs := command.MustHandle(ctx, bus, CreateProductCmd, func(ctx command.Ctx[CreateProductPayload]) error {
		return products.Use(ctx, ctx.AggregateID(), func(p *Product) error {
			pl := ctx.Payload()
			return p.Create(pl.Name, pl.Price, pl.Stock)
		})
	})

	renameErrs := command.MustHandle(ctx, bus, RenameProductCmd, func(ctx command.Ctx[string]) error {
		return products.Use(ctx, ctx.AggregateID(), func(p *Product) error {
			return p.Rename(ctx.Payload())
		})
	})

	priceErrs := command.MustHandle(ctx, bus, ChangePriceCmd, func(ctx command.Ctx[int]) error {
		return products.Use(ctx, ctx.AggregateID(), func(p *Product) error {
			return p.ChangePrice(ctx.Payload())
		})
	})

	stockErrs := command.MustHandle(ctx, bus, AdjustStockCmd, func(ctx command.Ctx[AdjustStockPayload]) error {
		return products.Use(ctx, ctx.AggregateID(), func(p *Product) error {
			pl := ctx.Payload()
			return p.AdjustStock(pl.Quantity, pl.Reason)
		})
	})

	return streams.FanInAll(createErrs, renameErrs, priceErrs, stockErrs)
}
```

## Wire It Up

Update `cmd/main.go`:

```go
import (
	// ... existing imports ...
	"github.com/modernice/goes/aggregate/repository"
	"github.com/modernice/goes/command/cmdbus"
	"github.com/yourname/shop"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	reg := codec.New()
	shop.RegisterProductEvents(reg)
	shop.RegisterProductCommands(reg) // [!code ++]

	store := eventstore.New()
	bus := eventbus.New()
	store = eventstore.WithBus(store, bus)

	repo := repository.New(store)
	products := repository.Typed(repo, shop.NewProduct)

	// Create the command bus.                                         // [!code ++]
	cbus := cmdbus.New[int](reg, bus)                                  // [!code ++]
	                                                                   // [!code ++]
	// Start handling product commands.                                // [!code ++]
	productErrs := shop.HandleProductCommands(ctx, cbus, products)     // [!code ++]
	go logErrors(productErrs)                                          // [!code ++]

	log.Println("Shop is running. Press Ctrl+C to stop.")
	<-ctx.Done()
}

func logErrors(errs <-chan error) {
	for err := range errs {
		log.Printf("Error: %v", err)
	}
}
```

## Dispatch a Command

Now you can create products by dispatching commands:

```go
id := uuid.New()
cmd := command.New(shop.CreateProductCmd, shop.CreateProductPayload{
	Name:  "Wireless Mouse",
	Price: 2999,
	Stock: 50,
}, command.Aggregate(shop.ProductAggregate, id))

if err := cbus.Dispatch(ctx, cmd.Any()); err != nil {
	log.Fatal(err)
}
```

## Understanding the Command Flow

```
Dispatch(CreateProductCmd)
    │
    ▼
Command Bus (routes to handler)
    │
    ▼
command.MustHandle callback
    │
    ▼
products.Use(ctx, id, func(p *Product) error {
    return p.Create(...)
})
    │
    ▼
1. Fetch product (replay events)
2. Call p.Create() (validates + raises event)
3. Save product (persist new events)
```

## `command.Ctx[P]`

The command context provides:
- `ctx.Payload()` — the typed command payload
- `ctx.AggregateID()` — the UUID of the target aggregate
- `ctx.AggregateName()` — the name of the target aggregate
- It also embeds `context.Context`, so you can pass it to any function that takes a context

## Error Channels

Command handlers return error channels (`<-chan error`). Errors are sent asynchronously as commands are handled. You're responsible for draining these channels — typically by logging or forwarding errors.

`streams.FanInAll` merges multiple error channels into one.

## Next

Let's build our second aggregate. In the [next chapter](./07-order-aggregate), we'll create the Order.
