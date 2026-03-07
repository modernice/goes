# 10. Projections

Aggregates are optimized for writes — they enforce business rules and record events. But reads are a different story. Replaying an entire event stream just to display a product list would be slow and wasteful.

**Projections** solve this by maintaining pre-built read models that update automatically as events flow through the system.

## The Product Catalog

Create `catalog.go` — a projection that maintains a list of all products:

```go
package shop

import (
	"context"
	"sync"

	"github.com/google/uuid"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/projection"
	"github.com/modernice/goes/projection/schedule"
)

// ProductView is the read model for a single product.
type ProductView struct {
	ID    uuid.UUID
	Name  string
	Price int
	Stock int
}

// ProductCatalog is a projection that maintains a list of all products.
type ProductCatalog struct {
	*projection.Base

	mux      sync.RWMutex
	products map[uuid.UUID]ProductView
}

func NewProductCatalog() *ProductCatalog {
	c := &ProductCatalog{
		Base:     projection.New(),
		products: make(map[uuid.UUID]ProductView),
	}

	event.ApplyWith(c, c.productCreated, ProductCreated)
	event.ApplyWith(c, c.productRenamed, ProductRenamed)
	event.ApplyWith(c, c.stockAdjusted, StockAdjusted)
	event.ApplyWith(c, c.pricingSet, PricingSet) // from Pricing aggregate

	return c
}
```

### Event Appliers

```go
func (c *ProductCatalog) productCreated(evt ProductCreatedEvent) {
	id, _, _ := evt.Aggregate()
	data := evt.Data()

	c.products[id] = ProductView{
		ID:    id,
		Name:  data.Name,
		Stock: data.Stock,
	}
}

func (c *ProductCatalog) productRenamed(evt ProductRenamedEvent) {
	id, _, _ := evt.Aggregate()

	if p, ok := c.products[id]; ok {
		p.Name = evt.Data()
		c.products[id] = p
	}
}

func (c *ProductCatalog) stockAdjusted(evt StockAdjustedEvent) {
	id, _, _ := evt.Aggregate()

	if p, ok := c.products[id]; ok {
		p.Stock += evt.Data().Quantity
		c.products[id] = p
	}
}

// pricingSet handles events from the Pricing aggregate — projections
// can subscribe to events from multiple aggregates.
func (c *ProductCatalog) pricingSet(evt PricingSetEvent) {
	id, _, _ := evt.Aggregate()

	if p, ok := c.products[id]; ok {
		p.Price = evt.Data().Price
		c.products[id] = p
	}
}
```

### Query Methods

```go
func (c *ProductCatalog) All() []ProductView {
	c.mux.RLock()
	defer c.mux.RUnlock()

	views := make([]ProductView, 0, len(c.products))
	for _, p := range c.products {
		views = append(views, p)
	}
	return views
}

func (c *ProductCatalog) Find(id uuid.UUID) (ProductView, bool) {
	c.mux.RLock()
	defer c.mux.RUnlock()
	p, ok := c.products[id]
	return p, ok
}
```

## Subscribe to Events

The catalog needs to receive events. We use a **continuous schedule** — it subscribes to the event bus and creates projection jobs whenever relevant events are published.

Add a `Run` method:

```go
func (c *ProductCatalog) Run(ctx context.Context, bus event.Bus, store event.Store) (<-chan error, error) {
	s := schedule.Continuously(bus, store, c.RegisteredEvents())

	return s.Subscribe(ctx, func(job projection.Job) error {
		c.mux.Lock()
		defer c.mux.Unlock()
		return job.Apply(job, c)
	}, projection.Startup())
}
```

### What's Happening?

1. **`c.RegisteredEvents()`** returns the event names from the handlers registered via `event.ApplyWith` in the constructor, so you don't have to list them again.
2. **`schedule.Continuously`** creates a schedule that reacts to events published on the bus.
3. **`s.Subscribe`** starts listening. When events arrive, the schedule creates a `projection.Job`.
4. **`job.Apply(job, c)`** fetches the job's events and applies them to the catalog.
5. **`projection.Startup()`** triggers an initial projection run on startup, catching up on any events that were stored before the projection started.

## Wire It Up

In `cmd/main.go`:

```go
func main() {
	// ... existing setup ...

	catalog := shop.NewProductCatalog()                       // [!code ++]
	catalogErrs, err := catalog.Run(ctx, bus, store)          // [!code ++]
	if err != nil {                                           // [!code ++]
		log.Fatal(err)                                        // [!code ++]
	}                                                         // [!code ++]
	go logErrors(catalogErrs)                                 // [!code ++]

	// ...
}
```

## How It Works

```
Event Store ──insert──▶ Event Bus ──publish──▶ Schedule
                                                  │
                                          creates Job
                                                  │
                                                  ▼
                                        job.Apply(job, catalog)
                                                  │
                                         fetches events from job
                                                  │
                                         applies to ProductCatalog
                                                  │
                                         updates read model
```

## Progressor

`*projection.Progressor` tracks which events have been applied by recording the time and ID of the last applied event. On subsequent runs, only events *after* the last progress point are fetched. This prevents duplicate processing.

A Progressor only makes sense when the projection's state is persisted to a database. In-memory projections like our `ProductCatalog` lose all state on restart and need to replay the entire event history anyway, so tracking progress would add no benefit.

### Shop Stats

Let's build a persisted projection that uses Progressor. Create `stats.go` — a singleton dashboard that counts products, orders, and revenue:

```go
package shop

import (
	"context"
	"sync"

	"github.com/google/uuid"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/persistence/model"
	"github.com/modernice/goes/projection"
	"github.com/modernice/goes/projection/schedule"
)

// ShopStatsID is a fixed UUID for the singleton stats model.
var ShopStatsID = uuid.MustParse("00000000-0000-0000-0000-000000000001")

// ShopStatsDTO holds the persisted shop metrics.
type ShopStatsDTO struct {
	ID            uuid.UUID `json:"id"`
	TotalProducts int       `json:"totalProducts"`
	TotalOrders   int       `json:"totalOrders"`
	TotalRevenue  int       `json:"totalRevenue"`
}

// ShopStats is a persisted projection that tracks shop-wide metrics.
type ShopStats struct {
	*projection.Base
	*projection.Progressor
	ShopStatsDTO

	mux sync.RWMutex
}

func NewShopStats(id uuid.UUID) *ShopStats {
	s := &ShopStats{
		Base:       projection.New(),
		Progressor: projection.NewProgressor(),
		ShopStatsDTO: ShopStatsDTO{
			ID: id,
		},
	}

	event.ApplyWith(s, s.productCreated, ProductCreated)
	event.ApplyWith(s, s.orderPlaced, OrderPlaced)
	event.ApplyWith(s, s.orderPaid, OrderPaid)

	return s
}

// ModelID implements model.Model.
func (s *ShopStats) ModelID() uuid.UUID {
	return s.ShopStatsDTO.ID
}

// ShopStatsRepository is the repository for the shop stats model.
type ShopStatsRepository = model.Repository[*ShopStats, uuid.UUID]
```

### Event Handlers

```go
func (s *ShopStats) productCreated(evt ProductCreatedEvent) {
	s.TotalProducts++
}

func (s *ShopStats) orderPlaced(evt OrderPlacedEvent) {
	s.TotalOrders++
}

func (s *ShopStats) orderPaid(evt OrderPaidEvent) {
	s.TotalRevenue += evt.Data()
}
```

### Running the Projection

```go
func RunShopStats(
	ctx context.Context,
	bus event.Bus,
	store event.Store,
	repo ShopStatsRepository,
) (<-chan error, error) {
	s := schedule.Continuously(bus, store, []string{
		ProductCreated, OrderPlaced, OrderPaid,
	})

	return s.Subscribe(ctx, func(job projection.Job) error {
		return repo.Use(job, ShopStatsID, func(stats *ShopStats) error {
			stats.mux.Lock()
			defer stats.mux.Unlock()
			return job.Apply(job, stats)
		})
	}, projection.Startup())
}
```

The important detail is inside `repo.Use`: the `stats` parameter is the model loaded from the repository — including its persisted `*Progressor`. When `job.Apply(job, stats)` runs, the job checks `stats.Progress()` and only fetches events *after* the last progress point. After applying, it updates the progress automatically. When `repo.Use` saves the model back, the updated progress is persisted too.

On restart, the cycle repeats: `repo.Use` loads the model with its saved progress, `job.Apply` skips all previously applied events, and only new events are processed. Without the Progressor, every restart would recount everything from the beginning.

### Wire It Up

Update `cmd/main.go`:

```go
func main() {
	// ... existing setup ...

	shopStatsRepo := memory.NewModelRepository[*shop.ShopStats, uuid.UUID]( // [!code ++]
		memory.ModelFactory(shop.NewShopStats),                               // [!code ++]
	)                                                                        // [!code ++]
	statsErrs, err := shop.RunShopStats(ctx, bus, store, shopStatsRepo)      // [!code ++]
	if err != nil {                                                          // [!code ++]
		log.Fatal(err)                                                       // [!code ++]
	}                                                                        // [!code ++]
	go logErrors(statsErrs)                                                  // [!code ++]

	// ...
}
```

## Debouncing

If many events are published in quick succession (e.g., bulk import), you don't want a separate job for each event. Use debouncing:

```go
s := schedule.Continuously(bus, store, eventNames,
	schedule.Debounce(time.Second),
)
```

This batches events within 1-second windows into a single job.

## Periodic Schedules

`schedule.Continuously` reacts to events in real time — great for projections that need to stay current. But some projections don't need immediate updates. A dashboard counter that refreshes every 30 seconds is perfectly fine.

For these, use `schedule.Periodically`:

```go
s := schedule.Periodically(store, 30*time.Second, []string{
	ProductCreated, OrderPlaced, OrderPaid,
})
```

A periodic schedule only needs the event store and an interval, not the event bus. On each tick, it queries the store for matching events and creates a job. Combined with a `Progressor`, each tick only fetches events that arrived since the last run.

Our ShopStats projection is a natural fit. Here's how `RunShopStats` would look with a periodic schedule instead:

```go
func RunShopStats(
	ctx context.Context,
	store event.Store,
	repo ShopStatsRepository,
) (<-chan error, error) {
	s := schedule.Periodically(store, 30*time.Second, []string{
		ProductCreated, OrderPlaced, OrderPaid,
	})

	return s.Subscribe(ctx, func(job projection.Job) error {
		return repo.Use(job, ShopStatsID, func(stats *ShopStats) error {
			stats.mux.Lock()
			defer stats.mux.Unlock()
			return job.Apply(job, stats)
		})
	}, projection.Startup())
}
```

Notice the bus parameter is gone — periodic schedules don't subscribe to the event bus. The `projection.Startup()` option still works: it triggers an initial catch-up run before the first tick.

### When to Use Which

| | Continuous | Periodic |
| --- | --- | --- |
| Trigger | Event published on bus | Timer tick |
| Latency | Real-time | Up to `interval` delay |
| Requires event bus | Yes | No |
| Best for | User-facing read models | Background analytics, reports |

## The Rebuild Approach

The ProductCatalog applies events one by one — each event has its own handler. This works, but notice how the appliers essentially duplicate the aggregate's state logic. Every time you add a new event to the Product aggregate, you also need to add a handler to the projection.

A simpler alternative: when events arrive, fetch the full aggregate from the event store and replace the entire read model. No per-event handlers needed.

### Order Summaries

Create `order_summary.go` — a persisted projection that maintains a read model for each order:

```go
package shop

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/helper/streams"
	"github.com/modernice/goes/persistence/model"
	"github.com/modernice/goes/projection"
	"github.com/modernice/goes/projection/schedule"
)

// OrderSummary is the read model for a single order.
// It enriches the order data with the customer's name.
type OrderSummary struct {
	OrderID      uuid.UUID   `json:"orderId"`
	CustomerID   uuid.UUID   `json:"customerId"`
	CustomerName string      `json:"customerName"`
	Items        []LineItem  `json:"items"`
	Status       OrderStatus `json:"status"`
	Total        int         `json:"total"`
}

func OrderSummaryOf(id uuid.UUID) *OrderSummary {
	return &OrderSummary{
		OrderID: id,
		Items:   make([]LineItem, 0),
	}
}

// ModelID implements model.Model.
func (s *OrderSummary) ModelID() uuid.UUID {
	return s.OrderID
}

// OrderSummaryRepository is the repository for order summaries.
type OrderSummaryRepository = model.Repository[*OrderSummary, uuid.UUID]
```

A few things are different from the ProductCatalog:

- No `*projection.Base` — we always rebuild from the aggregate, so there are no per-event appliers to register.
- The struct implements `ModelID()` to satisfy the `model.Model` interface.
- We define a `model.Repository` type alias for the projection's persistence layer.

### The Projector

```go
// OrderSummaryProjector maintains order summary read models.
type OrderSummaryProjector struct {
	customers CustomerRepository
	orders    OrderRepository
	summaries OrderSummaryRepository
}

func NewOrderSummaryProjector(customers CustomerRepository, orders OrderRepository, summaries OrderSummaryRepository) *OrderSummaryProjector {
	return &OrderSummaryProjector{
		customers: customers,
		orders:    orders,
		summaries: summaries,
	}
}

func (p *OrderSummaryProjector) Run(ctx context.Context, bus event.Bus, store event.Store) (<-chan error, error) {
	s := schedule.Continuously(bus, store, OrderEvents[:], schedule.Debounce(time.Second))

	return s.Subscribe(ctx, func(job projection.Job) error {
		refs, errs, err := job.Aggregates(job, OrderAggregate)
		if err != nil {
			return fmt.Errorf("extract aggregates: %w", err)
		}

		return streams.Walk(job, func(ref aggregate.Ref) error {
			return p.summaries.Use(job, ref.ID, func(summary *OrderSummary) error {
				order, err := p.orders.Fetch(job, ref.ID)
				if err != nil {
					return fmt.Errorf("fetch order: %w", err)
				}

				customer, err := p.customers.Fetch(job, order.CustomerID)
				if err != nil {
					return fmt.Errorf("fetch customer: %w", err)
				}

				summary.CustomerID = order.CustomerID
				summary.CustomerName = customer.CustomerDTO.Name
				summary.Items = order.Items
				summary.Status = order.Status
				summary.Total = order.Total

				return nil
			})
		}, refs, errs)
	}, projection.Startup())
}
```

Instead of writing an event handler for every Order event, we:

1. **`job.Aggregates(job, OrderAggregate)`** — extract the unique order aggregate refs from the job's events.
2. **`streams.Walk`** — iterate each affected order.
3. **`summaries.Use`** — atomically fetch the projection, modify it, and save it back.
4. Inside `Use`, we fetch the **full Order aggregate** and the associated **Customer** to build the summary — combining data that lives in separate aggregates.

Adding new events to the Order aggregate (e.g., `OrderRefunded`) requires zero changes to this projection — it always reads the current aggregate state.

### Wire It Up

Update `cmd/main.go`:

```go
import (
	// ... existing imports ...
	"github.com/modernice/goes/backend/memory"
)

func main() {
	// ... existing setup ...

	orderSummaries := memory.NewModelRepository[*shop.OrderSummary, uuid.UUID]( // [!code ++]
		memory.ModelFactory(shop.OrderSummaryOf),                               // [!code ++]
	)                                                                            // [!code ++]
	                                                                             // [!code ++]
	summaryProjector := shop.NewOrderSummaryProjector(customers, orders, orderSummaries) // [!code ++]
	summaryErrs, err := summaryProjector.Run(ctx, bus, store)               // [!code ++]
	if err != nil {                                                              // [!code ++]
		log.Fatal(err)                                                           // [!code ++]
	}                                                                            // [!code ++]
	go logErrors(summaryErrs)                                                    // [!code ++]

	// ...
}
```

`memory.ModelFactory(shop.OrderSummaryOf)` tells the repository to create new models using `OrderSummaryOf` instead of returning an error when a model doesn't exist yet.

## When to Rebuild

| | Event-by-Event | Rebuild from Aggregate |
| --- | --- | --- |
| Projection logic | One handler per event | One handler per aggregate |
| Complexity growth | Grows with each new event | Stays constant |
| State consistency | Must mirror aggregate logic | Always matches aggregate |
| Best for | Simple in-memory projections | Persisted and complex projections |

With a continuous schedule, the event-by-event approach is actually more efficient — events arrive directly from the bus and are never re-fetched from the store. The rebuild approach fetches the full aggregate from the store on every job. The exceptions are startup projections (via `projection.Startup()`) and periodic schedules, which always read from the store regardless of approach. But the difference rarely matters in practice, and the rebuild approach's simplicity usually wins. When in doubt, prefer rebuild.

The rebuild approach becomes especially important for multi-aggregate projections — read models that combine data from multiple aggregates. With event-by-event handling, you'd need to write handlers for events from every aggregate involved, figure out which aggregate each event belongs to, and somehow initialize or fetch the right state for each one. With the rebuild approach, you just fetch whatever aggregates you need and copy their state. The wiring stays simple regardless of how many aggregates are involved.

## Multi-Aggregate Projections

Some read models need data from multiple aggregates. An order history view might show the customer's name alongside their orders — combining data from both the Customer and Order aggregates.

### Customer Order History

Create `order_history.go`:

```go
package shop

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/helper/streams"
	"github.com/modernice/goes/persistence/model"
	"github.com/modernice/goes/projection"
	"github.com/modernice/goes/projection/schedule"
)

// CustomerOrderHistory is a read model combining customer and order data.
type CustomerOrderHistory struct {
	CustomerID   uuid.UUID      `json:"customerId"`
	CustomerName string         `json:"customerName"`
	Orders       []OrderSummary `json:"orders"`
}

func OrderHistoryOf(customerID uuid.UUID) *CustomerOrderHistory {
	return &CustomerOrderHistory{
		CustomerID: customerID,
		Orders:     make([]OrderSummary, 0),
	}
}

// ModelID implements model.Model.
func (h *CustomerOrderHistory) ModelID() uuid.UUID {
	return h.CustomerID
}

// CustomerOrderHistoryRepository is the repository for customer order histories.
type CustomerOrderHistoryRepository = model.Repository[*CustomerOrderHistory, uuid.UUID]
```

### The Projector

The key difference from single-aggregate projections is the routing. We call `job.Aggregates(job)` **without a filter** to get refs for all aggregate types, then switch on `ref.Name`:

```go
// CustomerOrderHistoryProjector maintains customer order history read models.
type CustomerOrderHistoryProjector struct {
	customers CustomerRepository
	orders    OrderRepository
	histories CustomerOrderHistoryRepository
}

func NewCustomerOrderHistoryProjector(
	customers CustomerRepository,
	orders OrderRepository,
	histories CustomerOrderHistoryRepository,
) *CustomerOrderHistoryProjector {
	return &CustomerOrderHistoryProjector{
		customers: customers,
		orders:    orders,
		histories: histories,
	}
}

func (p *CustomerOrderHistoryProjector) Run(ctx context.Context, bus event.Bus, store event.Store) (<-chan error, error) {
	s := schedule.Continuously(bus, store,
		append(CustomerEvents[:], OrderEvents[:]...),
		schedule.Debounce(time.Second),
	)

	return s.Subscribe(ctx, func(job projection.Job) error {
		refs, errs, err := job.Aggregates(job)
		if err != nil {
			return fmt.Errorf("extract aggregates: %w", err)
		}

		return streams.Walk(job, func(ref aggregate.Ref) error {
			switch ref.Name {
			case CustomerAggregate:
				return p.rebuildForCustomer(job, ref.ID)
			case OrderAggregate:
				return p.rebuildForOrder(job, ref.ID)
			}
			return nil
		}, refs, errs)
	}, projection.Startup())
}
```

### Rebuild Methods

When a Customer event arrives, we update the customer's name in their history:

```go
func (p *CustomerOrderHistoryProjector) rebuildForCustomer(ctx context.Context, customerID uuid.UUID) error {
	return p.histories.Use(ctx, customerID, func(h *CustomerOrderHistory) error {
		customer, err := p.customers.Fetch(ctx, customerID)
		if err != nil {
			return fmt.Errorf("fetch customer: %w", err)
		}

		h.CustomerName = customer.CustomerDTO.Name

		return nil
	})
}
```

When an Order event arrives, we first resolve the customer ID by fetching the order, then update that customer's history:

```go
func (p *CustomerOrderHistoryProjector) rebuildForOrder(ctx context.Context, orderID uuid.UUID) error {
	order, err := p.orders.Fetch(ctx, orderID)
	if err != nil {
		return fmt.Errorf("fetch order: %w", err)
	}

	return p.histories.Use(ctx, order.CustomerID, func(h *CustomerOrderHistory) error {
		// Refresh customer name.
		customer, err := p.customers.Fetch(ctx, h.CustomerID)
		if err != nil {
			return fmt.Errorf("fetch customer: %w", err)
		}
		h.CustomerName = customer.CustomerDTO.Name

		// Upsert the order in the history.
		summary := OrderSummary{
			OrderID:    order.ID,
			CustomerID: order.CustomerID,
			Items:      order.Items,
			Status:     order.Status,
			Total:      order.Total,
		}

		for i, o := range h.Orders {
			if o.OrderID == orderID {
				h.Orders[i] = summary
				return nil
			}
		}

		h.Orders = append(h.Orders, summary)
		return nil
	})
}
```

### Wire It Up

Update `cmd/main.go`:

```go
func main() {
	// ... existing setup ...

	orderHistories := memory.NewModelRepository[*shop.CustomerOrderHistory, uuid.UUID]( // [!code ++]
		memory.ModelFactory(shop.OrderHistoryOf),                                          // [!code ++]
	)                                                                                     // [!code ++]
	                                                                                      // [!code ++]
	historyProjector := shop.NewCustomerOrderHistoryProjector(                            // [!code ++]
		customers, orders, orderHistories,                                                // [!code ++]
	)                                                                                     // [!code ++]
	historyErrs, err := historyProjector.Run(ctx, bus, store)                        // [!code ++]
	if err != nil {                                                                       // [!code ++]
		log.Fatal(err)                                                                    // [!code ++]
	}                                                                                     // [!code ++]
	go logErrors(historyErrs)                                                             // [!code ++]

	// ...
}
```

## Next

Our application is feature-complete with in-memory backends. In the [next chapter](./11-backends), we'll swap them for production-ready MongoDB and NATS.
