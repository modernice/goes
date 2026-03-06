# 3. Events & State

Our Product can be created, but a real product needs more capabilities. Let's add events for renaming, changing the price, and adjusting stock.

## Add More Events

Update `product.go` to add the new event names, data types, and type aliases:

```go
// Event names.
const (
	ProductCreated = "shop.product.created"
	ProductRenamed = "shop.product.renamed"
	PriceChanged   = "shop.product.price_changed"
	StockAdjusted  = "shop.product.stock_adjusted"
)

// ProductEvents contains all Product event names.
var ProductEvents = [...]string{
	ProductCreated,
	ProductRenamed,
	PriceChanged,
	StockAdjusted,
}

// Event data types.
type ProductCreatedData struct {
	Name  string
	Price int
	Stock int
}

type StockAdjustedData struct {
	Quantity int
	Reason   string
}

// Event type aliases.
type (
	ProductCreatedEvent = event.Of[ProductCreatedData]
	ProductRenamedEvent = event.Of[string]
	PriceChangedEvent   = event.Of[int]
	StockAdjustedEvent  = event.Of[StockAdjustedData]
)
```

When event data is a single value, use the primitive type directly — no need for a wrapper struct. `ProductRenamed` carries just a `string` (the new name) and `PriceChanged` carries just an `int` (the new price).

::: tip Primitives vs. Structs
Using a primitive is fine when the event will only ever carry that one value. But keep in mind: if you later need to add fields to an event, a struct is easier to evolve. You can add fields to a struct and existing events in the store will still deserialize — the new fields just get their zero values. Changing the event data type itself (e.g., from `string` to a struct) is harder since stored events were serialized with the original type.
:::

Register the new appliers in the constructor:

```go
func NewProduct(id uuid.UUID) *Product {
	p := &Product{
		Base: aggregate.New(ProductAggregate, id),
		ProductDTO: ProductDTO{
			ID: id,
		},
	}

	event.ApplyWith(p, p.created, ProductCreated)
	event.ApplyWith(p, p.renamed, ProductRenamed)
	event.ApplyWith(p, p.priceChanged, PriceChanged)
	event.ApplyWith(p, p.stockAdjusted, StockAdjusted)

	return p
}
```

## Business Methods & Appliers

Each domain method validates input, raises an event, and is immediately followed by its applier. This keeps the cause and effect together:

```go
// Create initializes a new product.
func (p *Product) Create(name string, price, stock int) error {
	if p.Created() {
		return fmt.Errorf("product already created")
	}
	if name == "" {
		return fmt.Errorf("product name is required")
	}
	if price <= 0 {
		return fmt.Errorf("price must be positive")
	}
	if stock < 0 {
		return fmt.Errorf("stock cannot be negative")
	}
	aggregate.Next(p, ProductCreated, ProductCreatedData{
		Name:  name,
		Price: price,
		Stock: stock,
	})
	return nil
}

func (p *Product) created(evt ProductCreatedEvent) {
	data := evt.Data()
	p.Name = data.Name
	p.Price = data.Price
	p.Stock = data.Stock
}

// Rename changes the product's name.
func (p *Product) Rename(name string) error {
	if !p.Created() {
		return fmt.Errorf("product not created")
	}
	if name == "" {
		return fmt.Errorf("product name is required")
	}
	if name == p.Name {
		return nil // no change, no event
	}

	aggregate.Next(p, ProductRenamed, name)
	return nil
}

func (p *Product) renamed(evt ProductRenamedEvent) {
	p.Name = evt.Data()
}

// ChangePrice updates the product's price.
func (p *Product) ChangePrice(price int) error {
	if !p.Created() {
		return fmt.Errorf("product not created")
	}
	if price <= 0 {
		return fmt.Errorf("price must be positive")
	}
	if price == p.Price {
		return nil
	}

	aggregate.Next(p, PriceChanged, price)
	return nil
}

func (p *Product) priceChanged(evt PriceChangedEvent) {
	p.Price = evt.Data()
}

// AdjustStock adds or removes stock.
func (p *Product) AdjustStock(quantity int, reason string) error {
	if !p.Created() {
		return fmt.Errorf("product not created")
	}
	newStock := p.Stock + quantity
	if newStock < 0 {
		return fmt.Errorf("insufficient stock (have %d, adjusting by %d)", p.Stock, quantity)
	}

	aggregate.Next(p, StockAdjusted, StockAdjustedData{
		Quantity: quantity,
		Reason:   reason,
	})
	return nil
}

func (p *Product) stockAdjusted(evt StockAdjustedEvent) {
	p.Stock += evt.Data().Quantity
}
```

::: tip No Event If No Change
Notice that `Rename` and `ChangePrice` return early if the value hasn't actually changed. Don't raise events that don't change state — they add noise to your event stream without value.
:::

## Understanding `event.Of[T]`

`event.Of[T]` is a generic event interface. The type parameter `T` is the event data type — it can be a struct, a primitive, or any serializable type. You get type-safe access to the event's payload:

```go
func (p *Product) priceChanged(evt PriceChangedEvent) {
	// evt.Data() returns int — no type assertions needed.
	p.Price = evt.Data()

	// You also have access to event metadata:
	// evt.ID()        — unique event UUID
	// evt.Name()      — "shop.product.price_changed"
	// evt.Time()      — when the event was created
	// evt.Aggregate() — (id, name, version) of the aggregate
}
```

## Event Naming Conventions

goes doesn't enforce naming conventions, but we recommend:

- **Past tense** — events describe what happened: `ProductCreated`, not `CreateProduct`
- **Dot notation** — `shop.product.created` groups events by domain and aggregate
- **Constants** — define event names as constants to avoid typos

## The Complete Product

Here's the full `product.go` at this point:

```go
package shop

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/event"
)

const ProductAggregate = "shop.product"

// Event names.
const (
	ProductCreated = "shop.product.created"
	ProductRenamed = "shop.product.renamed"
	PriceChanged   = "shop.product.price_changed"
	StockAdjusted  = "shop.product.stock_adjusted"
)

// ProductEvents contains all Product event names.
var ProductEvents = [...]string{
	ProductCreated,
	ProductRenamed,
	PriceChanged,
	StockAdjusted,
}

// Event data types.
type ProductCreatedData struct {
	Name  string
	Price int
	Stock int
}

type StockAdjustedData struct {
	Quantity int
	Reason   string
}

// Event type aliases.
type (
	ProductCreatedEvent = event.Of[ProductCreatedData]
	ProductRenamedEvent = event.Of[string]
	PriceChangedEvent   = event.Of[int]
	StockAdjustedEvent  = event.Of[StockAdjustedData]
)

// ProductDTO holds the read state of a product.
type ProductDTO struct {
	ID    uuid.UUID `json:"id"`
	Name  string    `json:"name"`
	Price int       `json:"price"`
	Stock int       `json:"stock"`
}

// Created reports whether the product has been initialized.
func (dto ProductDTO) Created() bool {
	return dto.Name != ""
}

// Product is an event-sourced product.
type Product struct {
	*aggregate.Base
	ProductDTO
}

// NewProduct creates a new Product aggregate with the given ID.
func NewProduct(id uuid.UUID) *Product {
	p := &Product{
		Base: aggregate.New(ProductAggregate, id),
		ProductDTO: ProductDTO{
			ID: id,
		},
	}

	event.ApplyWith(p, p.created, ProductCreated)
	event.ApplyWith(p, p.renamed, ProductRenamed)
	event.ApplyWith(p, p.priceChanged, PriceChanged)
	event.ApplyWith(p, p.stockAdjusted, StockAdjusted)

	return p
}

// Create initializes a new product.
func (p *Product) Create(name string, price, stock int) error {
	if p.Created() {
		return fmt.Errorf("product already created")
	}
	if name == "" {
		return fmt.Errorf("product name is required")
	}
	if price <= 0 {
		return fmt.Errorf("price must be positive")
	}
	if stock < 0 {
		return fmt.Errorf("stock cannot be negative")
	}
	aggregate.Next(p, ProductCreated, ProductCreatedData{
		Name:  name,
		Price: price,
		Stock: stock,
	})
	return nil
}

func (p *Product) created(evt ProductCreatedEvent) {
	data := evt.Data()
	p.Name = data.Name
	p.Price = data.Price
	p.Stock = data.Stock
}

// Rename changes the product's name.
func (p *Product) Rename(name string) error {
	if !p.Created() {
		return fmt.Errorf("product not created")
	}
	if name == "" {
		return fmt.Errorf("product name is required")
	}
	if name == p.Name {
		return nil
	}
	aggregate.Next(p, ProductRenamed, name)
	return nil
}

func (p *Product) renamed(evt ProductRenamedEvent) {
	p.Name = evt.Data()
}

// ChangePrice updates the product's price.
func (p *Product) ChangePrice(price int) error {
	if !p.Created() {
		return fmt.Errorf("product not created")
	}
	if price <= 0 {
		return fmt.Errorf("price must be positive")
	}
	if price == p.Price {
		return nil
	}
	aggregate.Next(p, PriceChanged, price)
	return nil
}

func (p *Product) priceChanged(evt PriceChangedEvent) {
	p.Price = evt.Data()
}

// AdjustStock adds or removes stock.
func (p *Product) AdjustStock(quantity int, reason string) error {
	if !p.Created() {
		return fmt.Errorf("product not created")
	}
	newStock := p.Stock + quantity
	if newStock < 0 {
		return fmt.Errorf("insufficient stock (have %d, adjusting by %d)", p.Stock, quantity)
	}
	aggregate.Next(p, StockAdjusted, StockAdjustedData{
		Quantity: quantity,
		Reason:   reason,
	})
	return nil
}

func (p *Product) stockAdjusted(evt StockAdjustedEvent) {
	p.Stock += evt.Data().Quantity
}
```

## Next

The event store needs to know how to serialize our event data. In the [next chapter](./04-codec-registry), we'll set up the codec registry.
