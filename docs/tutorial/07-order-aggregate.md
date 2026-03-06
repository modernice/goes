# 7. The Order Aggregate

Time to build the Order — the second aggregate in our e-commerce system. This reinforces the patterns from the Product and introduces a few new ideas: value objects, status tracking, and cross-aggregate references.

## Define the Order

Create `order.go`:

```go
package shop

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/repository"
	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/command"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/helper/streams"
)

const OrderAggregate = "shop.order"

// Event names.
const (
	OrderPlaced    = "shop.order.placed"
	OrderPaid      = "shop.order.paid"
	OrderCancelled = "shop.order.cancelled"
)

// OrderEvents contains all Order event names.
var OrderEvents = [...]string{
	OrderPlaced,
	OrderPaid,
	OrderCancelled,
}

// Event data types.
type OrderPlacedData struct {
	CustomerID uuid.UUID
	Items      []LineItem
}

// Event type aliases.
type (
	OrderPlacedEvent    = event.Of[OrderPlacedData]
	OrderPaidEvent      = event.Of[int]
	OrderCancelledEvent = event.Of[string]
)

// Value object — not an aggregate, just a data structure.
type LineItem struct {
	ProductID uuid.UUID
	Name      string
	Price     int
	Quantity  int
}

// Status tracks the order lifecycle.
type OrderStatus int

const (
	OrderStatusOpen OrderStatus = iota
	OrderStatusPaid
	OrderStatusCancelled
)

// OrderDTO holds the read state of an order.
type OrderDTO struct {
	ID         uuid.UUID   `json:"id"`
	CustomerID uuid.UUID   `json:"customerId"`
	Items      []LineItem  `json:"items"`
	Status     OrderStatus `json:"status"`
	Total      int         `json:"total"`
}

// Placed reports whether the order has been placed.
func (dto OrderDTO) Placed() bool {
	return len(dto.Items) > 0
}

// Open reports whether the order is open.
func (dto OrderDTO) Open() bool {
	return dto.Placed() && dto.Status == OrderStatusOpen
}

// Paid reports whether the order has been paid.
func (dto OrderDTO) Paid() bool {
	return dto.Status == OrderStatusPaid
}

// Cancelled reports whether the order has been cancelled.
func (dto OrderDTO) Cancelled() bool {
	return dto.Status == OrderStatusCancelled
}

// Order is an event-sourced order.
type Order struct {
	*aggregate.Base
	OrderDTO
}

func NewOrder(id uuid.UUID) *Order {
	o := &Order{
		Base: aggregate.New(OrderAggregate, id),
		OrderDTO: OrderDTO{
			ID:    id,
			Items: make([]LineItem, 0),
		},
	}

	event.ApplyWith(o, o.placed, OrderPlaced)
	event.ApplyWith(o, o.paid, OrderPaid)
	event.ApplyWith(o, o.cancelled, OrderCancelled)

	return o
}
```

## Business Methods & Appliers

```go
// Place creates an order with the given items.
func (o *Order) Place(customerID uuid.UUID, items []LineItem) error {
	if o.Placed() {
		return fmt.Errorf("order already placed")
	}
	if len(items) == 0 {
		return fmt.Errorf("order must have at least one item")
	}
	if customerID == uuid.Nil {
		return fmt.Errorf("customer ID is required")
	}

	aggregate.Next(o, OrderPlaced, OrderPlacedData{
		CustomerID: customerID,
		Items:      items,
	})
	return nil
}

func (o *Order) placed(evt OrderPlacedEvent) {
	data := evt.Data()
	o.CustomerID = data.CustomerID
	o.Items = data.Items
	o.Status = OrderStatusOpen

	total := 0
	for _, item := range data.Items {
		total += item.Price * item.Quantity
	}
	o.Total = total
}

// Pay marks the order as paid.
func (o *Order) Pay(amount int) error {
	if !o.Open() {
		return fmt.Errorf("can only pay for open orders")
	}
	if amount != o.Total {
		return fmt.Errorf("payment amount %d does not match total %d", amount, o.Total)
	}

	aggregate.Next(o, OrderPaid, amount)
	return nil
}

func (o *Order) paid(evt OrderPaidEvent) {
	o.Status = OrderStatusPaid
}

// Cancel cancels the order.
func (o *Order) Cancel(reason string) error {
	if !o.Open() {
		return fmt.Errorf("can only cancel open orders")
	}

	aggregate.Next(o, OrderCancelled, reason)
	return nil
}

func (o *Order) cancelled(evt OrderCancelledEvent) {
	o.Status = OrderStatusCancelled
}
```

Notice the validation in `Pay` and `Cancel` — the aggregate protects its invariants. You can't pay for a cancelled order, and you can't cancel an order that's already been paid.

## Commands

```go
const (
	PlaceOrderCmd  = "shop.order.place"
	PayOrderCmd    = "shop.order.pay"
	CancelOrderCmd = "shop.order.cancel"
)

type PlaceOrderPayload struct {
	CustomerID uuid.UUID
	Items      []LineItem
}

```

## Registration

```go
func RegisterOrderEvents(r codec.Registerer) {
	codec.Register[OrderPlacedData](r, OrderPlaced)
	codec.Register[int](r, OrderPaid)
	codec.Register[string](r, OrderCancelled)
}

func RegisterOrderCommands(r codec.Registerer) {
	codec.Register[PlaceOrderPayload](r, PlaceOrderCmd)
	codec.Register[int](r, PayOrderCmd)
	codec.Register[string](r, CancelOrderCmd)
}
```

## Command Handlers

```go
// OrderRepository is the typed repository for orders.
type OrderRepository = aggregate.TypedRepository[*Order]

func HandleOrderCommands(
	ctx context.Context,
	bus command.Bus,
	orders OrderRepository,
	products ProductRepository,
) <-chan error {
	placeErrs := command.MustHandle(ctx, bus, PlaceOrderCmd, func(ctx command.Ctx[PlaceOrderPayload]) error {
		pl := ctx.Payload()

		return orders.Use(ctx, ctx.AggregateID(), func(o *Order) error {
			// Adjust stock first — if any product has insufficient
			// stock, the order is never placed.
			for _, item := range pl.Items {
				if err := products.Use(ctx, item.ProductID, func(p *Product) error {
					return p.AdjustStock(-item.Quantity, "ordered")
				}); err != nil {
					return err
				}
			}

			return o.Place(pl.CustomerID, pl.Items)
		})
	})

	payErrs := command.MustHandle(ctx, bus, PayOrderCmd, func(ctx command.Ctx[int]) error {
		return orders.Use(ctx, ctx.AggregateID(), func(o *Order) error {
			return o.Pay(ctx.Payload())
		})
	})

	cancelErrs := command.MustHandle(ctx, bus, CancelOrderCmd, func(ctx command.Ctx[string]) error {
		return orders.Use(ctx, ctx.AggregateID(), func(o *Order) error {
			return o.Cancel(ctx.Payload())
		})
	})

	return streams.FanInAll(placeErrs, payErrs, cancelErrs)
}
```

### Multi-Aggregate Commands

The `PlaceOrderCmd` handler operates across two aggregates — it places the order *and* adjusts stock for each product. This is done by nesting `Use` calls:

1. The outer `orders.Use` loads the Order aggregate.
2. For each line item, the inner `products.Use` loads the Product and calls `AdjustStock`.
3. If any `AdjustStock` fails (e.g. insufficient stock), the error propagates up and the Order is never placed.
4. Only after all stock is reserved does `o.Place(...)` raise the `OrderPlaced` event.

`Use` only saves when the function returns `nil`. This means you get all-or-nothing semantics within a single command handler — no partial state is ever persisted.

> [!NOTE]
> This works because everything runs in the same process. For operations spanning separate services, you'd coordinate with asynchronous messaging instead.

## Wire Into main.go

```go
func main() {
	// ... existing setup ...

	shop.RegisterProductEvents(reg)
	shop.RegisterProductCommands(reg)
	shop.RegisterOrderEvents(reg)     // [!code ++]
	shop.RegisterOrderCommands(reg)   // [!code ++]

	// ...

	products := repository.Typed(repo, shop.NewProduct)
	orders := repository.Typed(repo, shop.NewOrder)  // [!code ++]

	cbus := cmdbus.New[int](reg, bus)

	productErrs := shop.HandleProductCommands(ctx, cbus, products)
	orderErrs := shop.HandleOrderCommands(ctx, cbus, orders, products)  // [!code ++]

	go logErrors(productErrs)
	go logErrors(orderErrs)  // [!code ++]

	// ...
}
```

## Patterns to Notice

1. **Same structure as Product** — aggregates follow the same pattern: DTO, events, type aliases, constructor with `event.ApplyWith`, business methods with colocated appliers, registration, command handlers. Once you know the pattern, adding aggregates is fast.

2. **Cross-aggregate references** — The order references a customer by ID (`CustomerID uuid.UUID`), not by embedding a Customer struct. Aggregates reference each other by ID — never by embedding.

3. **Status as guard** — The `Pay` and `Cancel` methods check the order's status before allowing the operation. This is how aggregates enforce business rules.

## Next

Let's add our third aggregate — the [Customer](./08-customer-aggregate).
