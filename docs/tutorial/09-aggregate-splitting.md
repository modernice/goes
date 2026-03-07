# 9. Aggregate Splitting

Our Product aggregate handles name, price, stock, and soon it might need discounts, bulk pricing, and time-based promotions. Rather than cramming all of that into one aggregate, we can split pricing into its own aggregate that shares the product's UUID.

This pattern — multiple aggregates sharing the same ID — keeps each aggregate small and focused while still representing the same domain entity.

## Why Split?

Pricing is becoming its own concern. Discounts, promotional pricing, price history, currency support — none of that belongs in the Product aggregate alongside name and stock management. Rather than bloating Product, we give pricing its own aggregate.

Splitting lets each aggregate focus on one concern:

- **Product** — identity, name, stock
- **Pricing** — default price, discounts, promotions

Both use the same product UUID, but they have completely separate event streams and version histories.

## Refactor the Product

Before creating the Pricing aggregate, clean up the Product. Remove everything related to pricing — the Pricing aggregate will own it entirely.

In `product.go`, start by removing `Price` from the data structures:

```go
type ProductCreatedData struct {
	Name  string
	Price int // [!code --]
	Stock int
}
```

```go
type ProductDTO struct {
	ID    uuid.UUID `json:"id"`
	Name  string    `json:"name"`
	Price int       `json:"price"` // [!code --]
	Stock int       `json:"stock"`
}
```

Update the `Create` method — it no longer accepts a price:

```go
func (p *Product) Create(name string, price, stock int) error { // [!code --]
func (p *Product) Create(name string, stock int) error { // [!code ++]
	if p.Created() {
		return fmt.Errorf("product already created")
	}
	if name == "" {
		return fmt.Errorf("product name is required")
	}
	if price <= 0 {                                // [!code --]
		return fmt.Errorf("price must be positive") // [!code --]
	}                                              // [!code --]
	if stock < 0 {
		return fmt.Errorf("stock cannot be negative")
	}

	aggregate.Next(p, ProductCreated, ProductCreatedData{
		Name:  name,
		Price: price, // [!code --]
		Stock: stock,
	})

	return nil
}
```

```go
func (p *Product) created(evt ProductCreatedEvent) {
	data := evt.Data()
	p.Name = data.Name
	p.Price = data.Price // [!code --]
	p.Stock = data.Stock
}
```

Remove the `PriceChanged` event, the `ChangePrice` method, and the `ChangePriceCmd` command:

```go
const (
	ProductCreated = "shop.product.created"
	ProductRenamed = "shop.product.renamed"
	PriceChanged   = "shop.product.price_changed" // [!code --]
	StockAdjusted  = "shop.product.stock_adjusted"
)

var ProductEvents = [...]string{
	ProductCreated,
	ProductRenamed,
	PriceChanged, // [!code --]
	StockAdjusted,
}
```

```go
type (
	ProductCreatedEvent = event.Of[ProductCreatedData]
	ProductRenamedEvent = event.Of[string]
	PriceChangedEvent   = event.Of[int] // [!code --]
	StockAdjustedEvent  = event.Of[StockAdjustedData]
)
```

```go
event.ApplyWith(p, p.created, ProductCreated)
event.ApplyWith(p, p.renamed, ProductRenamed)
event.ApplyWith(p, p.priceChanged, PriceChanged) // [!code --]
event.ApplyWith(p, p.stockAdjusted, StockAdjusted)
```

Remove the entire `ChangePrice` method and its applier:

```go
func (p *Product) ChangePrice(price int) error { // [!code --]
	if !p.Created() {                              // [!code --]
		return fmt.Errorf("product not created")   // [!code --]
	}                                              // [!code --]
	if price <= 0 {                                // [!code --]
		return fmt.Errorf("price must be positive") // [!code --]
	}                                              // [!code --]
	if price == p.Price {                          // [!code --]
		return nil                                 // [!code --]
	}                                              // [!code --]
	aggregate.Next(p, PriceChanged, price)         // [!code --]
	return nil                                     // [!code --]
}                                                  // [!code --]
                                                   // [!code --]
func (p *Product) priceChanged(evt PriceChangedEvent) { // [!code --]
	p.Price = evt.Data()                                // [!code --]
}                                                       // [!code --]
```

Remove from codec registration and commands:

```go
func RegisterProductEvents(r codec.Registerer) {
	codec.Register[ProductCreatedData](r, ProductCreated)
	codec.Register[string](r, ProductRenamed)
	codec.Register[int](r, PriceChanged) // [!code --]
	codec.Register[StockAdjustedData](r, StockAdjusted)
}
```

```go
const (
	CreateProductCmd = "shop.product.create"
	RenameProductCmd = "shop.product.rename"
	ChangePriceCmd   = "shop.product.change_price" // [!code --]
	AdjustStockCmd   = "shop.product.adjust_stock"
)

func RegisterProductCommands(r codec.Registerer) {
	codec.Register[CreateProductPayload](r, CreateProductCmd)
	codec.Register[string](r, RenameProductCmd)
	codec.Register[int](r, ChangePriceCmd) // [!code --]
	codec.Register[AdjustStockPayload](r, AdjustStockCmd)
}
```

```go
func HandleProductCommands(
	ctx context.Context,
	bus command.Bus,
	products ProductRepository,
) <-chan error {
	createErrs := command.MustHandle(ctx, bus, CreateProductCmd, func(ctx command.Ctx[CreateProductPayload]) error {
		return products.Use(ctx, ctx.AggregateID(), func(p *Product) error {
			pl := ctx.Payload()
			return p.Create(pl.Name, pl.Price, pl.Stock) // [!code --]
			return p.Create(pl.Name, pl.Stock) // [!code ++]
		})
	})

	renameErrs := command.MustHandle(ctx, bus, RenameProductCmd, func(ctx command.Ctx[string]) error {
		return products.Use(ctx, ctx.AggregateID(), func(p *Product) error {
			return p.Rename(ctx.Payload())
		})
	})

	priceErrs := command.MustHandle(ctx, bus, ChangePriceCmd, func(ctx command.Ctx[int]) error { // [!code --]
		return products.Use(ctx, ctx.AggregateID(), func(p *Product) error {                     // [!code --]
			return p.ChangePrice(ctx.Payload())                                                  // [!code --]
		})                                                                                       // [!code --]
	})                                                                                           // [!code --]

	stockErrs := command.MustHandle(ctx, bus, AdjustStockCmd, func(ctx command.Ctx[AdjustStockPayload]) error {
		return products.Use(ctx, ctx.AggregateID(), func(p *Product) error {
			pl := ctx.Payload()
			return p.AdjustStock(pl.Quantity, pl.Reason)
		})
	})

	return streams.FanInAll(createErrs, renameErrs, priceErrs, stockErrs) // [!code --]
	return streams.FanInAll(createErrs, renameErrs, stockErrs) // [!code ++]
}
```

`CreateProductPayload` still has a `Price` field — it represents the caller's intent. The handler routes it to the Pricing aggregate ([below](#coordinating-split-aggregates)).

## Define the Pricing Aggregate

Create a new file `pricing.go`. Even though Pricing is closely related to Product, it's a separate aggregate with its own events, commands, and business logic — putting it in its own file keeps both files manageable.

```go
package shop

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/command"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/helper/streams"
)

const PricingAggregate = "shop.pricing"

// Event names.
const (
	PricingSet    = "shop.pricing.set"
	DiscountAdded = "shop.pricing.discount_added"
)

// PricingEvents contains all Pricing event names.
var PricingEvents = [...]string{
	PricingSet,
	DiscountAdded,
}

// Event data types.
type PricingSetData struct {
	Price int
}

type Discount struct {
	Label   string
	Percent int
}

func (d Discount) Validate() error {
	if d.Label == "" {
		return fmt.Errorf("discount label is required")
	}
	if d.Percent <= 0 || d.Percent > 100 {
		return fmt.Errorf("discount percent must be between 1 and 100")
	}
	return nil
}

// Event type aliases.
type (
	PricingSetEvent    = event.Of[PricingSetData]
	DiscountAddedEvent = event.Of[Discount]
)

// PricingDTO holds the read state of a product's pricing.
type PricingDTO struct {
	ProductID    uuid.UUID  `json:"productId"`
	DefaultPrice int        `json:"defaultPrice"`
	Discounts    []Discount `json:"discounts"`
}

// Pricing manages the pricing of a product.
type Pricing struct {
	*aggregate.Base
	PricingDTO
}

// PricingOf creates a Pricing aggregate for the given product.
// It shares the product's UUID — same ID, different aggregate name.
// The "Of" suffix signals that this is a split aggregate.
func PricingOf(productID uuid.UUID) *Pricing {
	p := &Pricing{
		Base: aggregate.New(PricingAggregate, productID),
		PricingDTO: PricingDTO{
			ProductID: productID,
			Discounts: make([]Discount, 0),
		},
	}

	event.ApplyWith(p, p.pricingSet, PricingSet)
	event.ApplyWith(p, p.discountAdded, DiscountAdded)

	return p
}
```

The key line is `aggregate.New(PricingAggregate, productID)`. The aggregate name is `"shop.pricing"`, but the ID is the product's UUID. The event store queries events by both name and ID, so `Product` and `Pricing` have completely separate event streams even though they share the same UUID.

## Business Methods

```go
// SetPrice sets the default price.
func (p *Pricing) SetPrice(price int) error {
	if price < 0 {
		return fmt.Errorf("price cannot be negative")
	}
	if price == p.DefaultPrice {
		return nil
	}
	aggregate.Next(p, PricingSet, PricingSetData{Price: price})
	return nil
}

func (p *Pricing) pricingSet(evt PricingSetEvent) {
	p.DefaultPrice = evt.Data().Price
}

// AddDiscount adds a percentage discount.
func (p *Pricing) AddDiscount(d Discount) error {
	if err := d.Validate(); err != nil {
		return err
	}
	aggregate.Next(p, DiscountAdded, d)
	return nil
}

func (p *Pricing) discountAdded(evt DiscountAddedEvent) {
	p.Discounts = append(p.Discounts, evt.Data())
}
```

## Registration

```go
func RegisterPricingEvents(r codec.Registerer) {
	codec.Register[PricingSetData](r, PricingSet)
	codec.Register[Discount](r, DiscountAdded)
}
```

## Commands

```go
const (
	SetPriceCmd    = "shop.pricing.set_price"
	AddDiscountCmd = "shop.pricing.add_discount"
)

func RegisterPricingCommands(r codec.Registerer) {
	codec.Register[int](r, SetPriceCmd)
	codec.Register[Discount](r, AddDiscountCmd)
}
```

## Command Handlers

```go
// PricingRepository is the typed repository for pricing.
type PricingRepository = aggregate.TypedRepository[*Pricing]

func HandlePricingCommands(
	ctx context.Context,
	bus command.Bus,
	pricing PricingRepository,
) <-chan error {
	setErrs := command.MustHandle(ctx, bus, SetPriceCmd, func(ctx command.Ctx[int]) error {
		return pricing.Use(ctx, ctx.AggregateID(), func(p *Pricing) error {
			return p.SetPrice(ctx.Payload())
		})
	})

	discountErrs := command.MustHandle(ctx, bus, AddDiscountCmd, func(ctx command.Ctx[Discount]) error {
		return pricing.Use(ctx, ctx.AggregateID(), func(p *Pricing) error {
			return p.AddDiscount(ctx.Payload())
		})
	})

	return streams.FanInAll(setErrs, discountErrs)
}
```

## Wire Into main.go

```go
func main() {
	// ... existing setup ...

	shop.RegisterProductEvents(eventReg)
	shop.RegisterOrderEvents(eventReg)
	shop.RegisterCustomerEvents(eventReg)
	shop.RegisterPricingEvents(eventReg)     // [!code ++]

	shop.RegisterProductCommands(cmdReg)
	shop.RegisterOrderCommands(cmdReg)
	shop.RegisterCustomerCommands(cmdReg)
	shop.RegisterPricingCommands(cmdReg)   // [!code ++]

	// ...

	products := repository.Typed(repo, shop.NewProduct)
	orders := repository.Typed(repo, shop.NewOrder)
	pricing := repository.Typed(repo, shop.PricingOf) // [!code ++]

	productErrs := shop.HandleProductCommands(ctx, cbus, products)
	orderErrs := shop.HandleOrderCommands(ctx, cbus, orders, products)
	customerErrs := shop.HandleCustomerCommands(ctx, cbus, customers)
	pricingErrs := shop.HandlePricingCommands(ctx, cbus, pricing) // [!code ++]

	go logErrors(streams.FanInAll(productErrs, orderErrs, customerErrs, pricingErrs))

	// ...
}
```

## How It Works

Here's what the event store looks like after creating a product and setting its pricing:

```
Product UUID: 550e8400-...

"shop.product" events:
  v1  ProductCreated { Name: "Mouse", Stock: 50 }

"shop.pricing" events:
  v1  PricingSet { Price: 2999 }
  v2  DiscountAdded { Discount: { Label: "Summer Sale", Percent: 10 } }
```

Each aggregate has its own version sequence starting from 1. When you fetch them:

```go
// Replays only "shop.product" events for this UUID
products.Fetch(ctx, productID) // → Product at v1

// Replays only "shop.pricing" events for this UUID
pricing.Fetch(ctx, productID)  // → Pricing at v2
```

## Coordinating Split Aggregates

When creating a product, you might want to set up both the Product and its Pricing in one command handler. Use nested `repo.Use` calls — the same pattern we used in the [Order chapter](./07-order-aggregate):

```go
command.MustHandle(ctx, bus, CreateProductCmd, func(ctx command.Ctx[CreateProductPayload]) error {
	pl := ctx.Payload()
	return products.Use(ctx, ctx.AggregateID(), func(p *Product) error {
		if err := p.Create(pl.Name, pl.Stock); err != nil {
			return err
		}
		return pricing.Use(ctx, ctx.AggregateID(), func(pr *Pricing) error {
			return pr.SetPrice(pl.Price)
		})
	})
})
```

If either fails, neither is saved.

## Patterns to Notice

1. **Same UUID, different names** — `PricingOf(productID)` creates a `"shop.pricing"` aggregate with the product's UUID. The event store keeps their event streams completely separate.

2. **`Of` constructors** — The core aggregate uses `NewProduct(id)`. Split aggregates use `PricingOf(productID)`, `ProductContentOf(productID)`, etc. The naming makes the relationship obvious.

3. **Clean separation** — Product owns identity, name, and stock. Pricing owns everything price-related. The `CreateProductPayload` still carries a price because it's the caller's intent — the handler routes it to the right aggregate.

4. **Projections recombine** — In the [next chapter](./10-projections), the `ProductCatalog` projection will subscribe to both `ProductEvents` and `PricingEvents` to build a complete view. The write model splits; the read model joins.

## Next

We have four aggregates producing events. In the [next chapter](./10-projections), we'll build read models that stay in sync.
