# SAGAs – Distributed Transactions

## Overview

A `Transaction` is a series or group of operations (mostly at the database
level) which should be performed as if they're a single operation. An example
for Transactions in the realm of Event-Sourcing is the insertion of
[Aggregate](./aggregates.md) changes into the
[Event Store](./events.md#event-store). During the runtime of an application an
Aggregate may records one or multiple [Events](./events.md) representing the
uncommitted changes to the Aggregate. Those Events must be inserted into
database as a single `Atomic Transaction`. This means that either all Events or
none are successfully inserted into the database. This is an important
requirement to ensure the integrity of an Aggregate.

When talking about Transactions we want them to have the following `ACID`
properties (taken from https://softwareengineering.stackexchange.com/a/289144):

- Atomicity - The transaction is a single, unbreakable unit. It cannot be
partially applied, or partially undone. It is either completely done, or not
done at all.

- Consistency - The application/database/whatever must be in a valid state both
before and after the transaction. If attempting to execute a transaction results
in an invalid state, then we must "rollback" to the last valid state.

- Isolation - Every transaction is separate from every other transaction. No two
transactions can ever "interleave". They are always executed one at a time (or
in a way that is indistinguishable from "one at a time").

- Durability - Once the transaction has been executed, it stays executed forever.
This one is mostly relevant for databases, where it means the change in data has
actually been committed to disk, so that the new data cannot be lost even if the
machine suddenly reboots.

These requirements are fulfilled when working with a single Aggregate whose
changes need to be committed to the Event Store, but not when working with
multiple Aggregates at once which may even spread across multiple services and
multiple processes. For this we need to be able to do so called
`Distributed Transactions`.

## Distributed Transactions

A `Distributed Transaction` is a Transaction that spans not only over a single
Aggregate but over multiple Aggregates and possibly even over multiple domain-
boundaries. This means that when working with two or more Aggregates whose
changes have to be committed into the Event Store all Aggregates must either
fail or succeed to be committed.

### Compensating Events

A solution to achieve Distributed Transactions is to use `Compensating Events`.
Instead of locking the Aggregates for the duration of the Transaction, we can
just raise more Events on these Aggregates which then bring the Aggregates back
into a valid state in the applications domain.

**Example:**

Given an ecommerce platform with an `Order` and a `Stock` service, each handling
Aggregates that are named after their service. So we have the Order service
which handles Aggregates and the Stock service which manages the stock of our
items.

Imagine the process of `Placing an Order` in our hypothetical ecommerce app. In
order for an Order to be placed, a few steps need to be executed:

- `Place the Order`
- `Check the Stock`
    - If the Stock is already empty, `Cancel the Order`
    - Otherwise `Update the Stock` & `Confirm the Order`

As you can see, Placing an Order doesn't act solely on the Order Aggregate, but
also on the Stock Aggregate. The Distributed Transaction that places an Order
either fails with a canceled Order or succeeds with a confirmed Order. 

Here, the Compensating Event to the `OrderPlaced` Event is the `OrderCanceled`
Event. Our Stock service might be subscribed to OrderPlaced Events and reacts to
them by checking the stock for the ordered item. It then may raise a
`StockReserved` Event which reserves part of the stock for the Order or a
`StockReserveFailed` Event which tells the Order service that the Order can not
be fulfilled. The Order service can then react accordingly to the Order services
Events and so on...

## SAGA

A `SAGA` coordinates and synchronizes the [Commands](./commands.md) that are
executed against a multiple Aggregates. In our ecommerce example we could have a
SAGA for placing Orders, let's call it the `PlaceOrderSAGA`. The PlaceOrderSAGA
has to do the following steps:

1. `Place the Order` in the Order service
2. Let the Order and Stock services handle the Events and Compensating Events
3. Wait for the `OrderConfirmed` or `OrderCanceled` Event to be published
4. Report the status to the caller

From these steps we can extrapolate the following rules:

- A SAGA has an initiating Command (in this case the `PlaceOrder` Command)
- A SAGA has one or many finalizing Events (in this case `OrderConfirmed` &
  `OrderCanceled`)
- 

## Design

### Idea #1 

```go
placeOrder := saga.New(
    command.PlaceOrder( // initiating command
        uuid.New(), // item id
        3, // quantity
        ... // customer data etc.
    ),

    // succeeds when an "order" is confirmed
    saga.SucceedsWhen("order", uuid.New(), event.OrderConfirmed),

    // fails when an "order" is canceled
    saga.FailsWhen("foo", uuid.New(), event.OrderCanceled),
)

err := saga.Execute(context.TODO(), placeOrder)
```

### Idea 2

```go
orderID := uuid.New()
itemID := uuid.New()

placeOrder := saga.New(
    saga.StartsWith(command.PlaceOrder(orderID, ...)),
    saga.On("order", orderID, event.OrderPlaced, func(evt event.Event) []saga.Command {
        data := evt.Data().(event.OrderPlacedData)
        return []saga.Command{
            command.ReserveStock(
                data.OrderID,
                data.ItemID,
                data.Quantity,
            ),
        } 
    }),
    saga.On("stock", itemID, event.StockReserved, func(evt event.Event) []saga.Command {
        data := evt.Data().(event.StockReservedData)
        return []saga.Command{
            command.ConfirmOrder(data.OrderID),
        }
    }),
    saga.On("stock", itemID, event.StockReserveFailed, func(evt event.Event) []saga.Command {
        data := evt.Data().(event.StockReserveFailedData)
        return []saga.Command{
            command.CancelOrder(data.OrderID, "empty stock"),
        }
    }),
    saga.SucceedsWhen("order", orderID, event.OrderConfirmed),
    saga.FailsWhen("order", orderID, event.OrderCanceled),
)

err := saga.Execute(context.TODO(), placeOrder)
```

### Idea 3

```go
orderID := uuid.New()
itemID := uuid.New()

placeOrder := saga.New(
    saga.StartsWith(command.PlaceOrder(itemID, 3, ...)),
    saga.SucceedsWhen("order", orderID, event.OrderConfirmed),
    saga.FailsWhen("order", orderID, event.OrderCanceled),
    saga.Compensate("stock", itemID, event.StockReserveFailed, command.CancelOrder(orderID, ...)),
)
```

### Idea 4

```go
orderID := uuid.New()
itemID := uuid.New()

placeOrder := saga.New(
    saga.StartsWith(command.PlaceOrder(itemID, 3, ...)),

    saga.SucceedsWhen(
        query.New(
            query.AggregateName("order"),
            query.AggregateID(orderID)),
            query.Name(event.OrderConfirmed),
            query.Time(time.After(stdtime.Now()),
        ),
    ),

    saga.SucceedsWith("order", orderID, event.OrderConfirmed),

    saga.FailsWhen(
        query.New(
            query.AggregateName("order"),
            query.AggregateID(orderID),
            query.Name(event.OrderCanceled),
            query.Time(time.After(stdtime.Now())),
        ),
    ),

    saga.FailsWith("order", orderID, event.OrderCanceled),

    saga.When(query.New(
        query.AggregateName("stock"),
        query.AggregateID(itemID)
        query.Name(event.StockReserveFailed),
        query.Time(time.After(stdtime.Now())),
    ), func(evt event.Event) []saga.Command {
        data := evt.Data().(event.StockReserveFailedData)
        return []saga.Command{
            command.CancelOrder(data.OrderID, "empty stock"),
        }
    }),
    
    saga.On(
        "stock",
        itemID,
        event.StockReserveFailed,
        func(evt event.Event) []saga.Command { ... }),
    ),
)
```

