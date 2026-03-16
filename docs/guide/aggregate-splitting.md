# Aggregate Splitting

When a single aggregate grows to handle too many concerns, you can split it into multiple aggregates that share the same UUID. Each aggregate type gets its own event stream, its own version history, and its own snapshot lifecycle.

> For a step-by-step walkthrough, see the [Tutorial](/tutorial/09-aggregate-splitting).

## Why Split?

A `Product` aggregate that manages name, price, stock, discounts, localized content, and SEO metadata will accumulate a long event stream. Some of those concerns change at very different rates — prices update daily, product names almost never.

Splitting gives you:

- **Fewer events per stream** — faster hydration, less to replay
- **Independent snapshots** — snapshot the frequently-changing aggregate without snapshotting the rest
- **Smaller consistency boundaries** — less contention, simpler business logic per aggregate
- **Independent evolution** — add events and methods to one aggregate without touching the others

## How It Works

goes identifies aggregates by a `(name, id)` tuple, not just an ID. Two aggregates with different names can share the same UUID and will have completely separate event streams:

```
Product UUID: 550e8400-...

"shop.product"  stream: ProductCreated → ProductRenamed → ...  (v1, v2, ...)
"shop.pricing"  stream: PricingSet → DiscountAdded → ...       (v1, v2, ...)
"shop.content"  stream: ContentLocalized → ...                 (v1, ...)
```

The event store queries by both name and ID, so `products.Fetch(ctx, id)` and `pricing.Fetch(ctx, id)` return entirely different event histories — even though the UUID is the same.

## Defining Split Aggregates

Start with the core aggregate — the one that owns the identity:

```go
const ProductAggregate = "shop.product"

type Product struct {
	*aggregate.Base
	ProductDTO
}

func NewProduct(id uuid.UUID) *Product {
	p := &Product{
		Base:       aggregate.New(ProductAggregate, id),
		ProductDTO: ProductDTO{ID: id},
	}
	event.ApplyWith(p, p.created, ProductCreated)
	event.ApplyWith(p, p.renamed, ProductRenamed)
	return p
}
```

Then define the split aggregate. Use an `Of` constructor to signal the relationship — `PricingOf(productID)` makes it clear that this aggregate belongs to a product:

```go
const PricingAggregate = "shop.pricing"

const (
	PricingSet    = "shop.pricing.set"
	DiscountAdded = "shop.pricing.discount_added"
)

type Discount struct {
	Label   string
	Percent int
}

type PricingDTO struct {
	ProductID    uuid.UUID  `json:"productId"`
	DefaultPrice int        `json:"defaultPrice"`
	Discounts    []Discount `json:"discounts"`
}

type Pricing struct {
	*aggregate.Base
	PricingDTO
}

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

The key line is `aggregate.New(PricingAggregate, productID)` — the name is `"shop.pricing"` but the ID is the product's UUID.

You can split as many facets as needed:

```go
func ProductContentOf(productID uuid.UUID) *ProductContent {
	// "shop.product_content" + same product UUID
	c := &ProductContent{
		Base: aggregate.New("shop.product_content", productID),
		ProductContentDTO: ProductContentDTO{
			Names:        make(map[string]string),
			Descriptions: make(map[string]string),
		},
	}
	event.ApplyWith(c, c.localized, ContentLocalized)
	return c
}
```

## Repositories

Each split aggregate gets its own typed repository. Both can be called with the same UUID:

```go
repo := repository.New(store)

products := repository.Typed(repo, shop.NewProduct)
pricing  := repository.Typed(repo, shop.PricingOf)
content  := repository.Typed(repo, shop.ProductContentOf)
```

`repository.Typed` binds to the aggregate name returned by the factory. When you call `pricing.Fetch(ctx, productID)`, it queries only `"shop.pricing"` events for that UUID.

## Independent Snapshots

Snapshots are keyed by `(name, id, version)`, so each split aggregate has its own snapshot lifecycle. You can snapshot the frequently-changing aggregate without touching the rest:

```go
// Snapshot pricing every 50 events — it changes often
pricingRepo := repository.New(store,
	repository.WithSnapshots(snapshots, snapshot.Every(50)),
)
pricing := repository.Typed(pricingRepo, shop.PricingOf)

// No snapshots for core product — it rarely changes
productRepo := repository.New(store)
products := repository.Typed(productRepo, shop.NewProduct)
```

See [Snapshots](/guide/snapshots) for configuration details.

## Commands Across Split Aggregates

A single command handler can coordinate across split aggregates using nested `repo.Use` calls:

```go
command.MustHandle(ctx, bus, CreateProductCmd, func(ctx command.Ctx[CreateProductPayload]) error {
	return products.Use(ctx, ctx.AggregateID(), func(p *Product) error {
		if err := p.Create(ctx.Payload().Name, ctx.Payload().Stock); err != nil {
			return err
		}
		// Set initial pricing on the same UUID
		return pricing.Use(ctx, ctx.AggregateID(), func(pr *Pricing) error {
			return pr.SetPrice(ctx.Payload().Price)
		})
	})
})
```

`Use` only saves when the callback returns `nil`, so if pricing fails, neither aggregate is persisted.

## Projections Recombine

Read models often need data from multiple split aggregates. A product catalog projection subscribes to events from both `Product` and `Pricing`:

```go
type ProductCatalog struct {
	*projection.Base
	products map[uuid.UUID]ProductView
}

func NewProductCatalog() *ProductCatalog {
	c := &ProductCatalog{
		Base:     projection.New(),
		products: make(map[uuid.UUID]ProductView),
	}

	event.ApplyWith(c, c.productCreated, shop.ProductCreated)
	event.ApplyWith(c, c.pricingSet, shop.PricingSet)
	event.ApplyWith(c, c.discountAdded, shop.DiscountAdded)

	return c
}
```

Recombination happens by registering handlers for events from each aggregate stream. The write model separates concerns into multiple aggregates, while the read model merges those concerns back into a projection tailored to the UI or query.

This pattern also works naturally with schedules: because the handlers are registered on `projection.Base`, `c.RegisteredEvents()` can be used when subscribing.

If you prefer a single dispatcher, you can also implement `ApplyEvent(event.Event)` manually. See [Projections](/guide/projections) for both handler styles and the tradeoffs.

## When to Split

**Split when:**
- One part of the aggregate changes much more frequently than another
- Different parts need different snapshot strategies
- The aggregate accumulates many event types across unrelated concerns
- Teams work on different aspects independently

**Don't split when:**
- The parts must be consistent within a single transaction (e.g., stock and reserved stock)
- The aggregate is small and simple — splitting adds complexity
- You're optimizing prematurely — start with one aggregate and split when the need arises

## Naming Conventions

| Aggregate | Constructor | Rationale |
| --- | --- | --- |
| Core entity | `NewProduct(id)` | Standard constructor — this is the primary aggregate |
| Split facet | `PricingOf(productID)` | `Of` suffix signals this is a split aggregate |
| Split facet | `ProductContentOf(productID)` | Same pattern — reads as "content of product" |
