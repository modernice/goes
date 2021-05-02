# ecommerce example

The application consists of 3 microservices: `Product`, `Stock` and `Order`.

## Product service

The `Product` service provides creation of new Products.

### Create a Product

In order to create a Product, dispatch a `product.Create()` Command:

```go
func createProduct(bus command.Bus, name string, unitPrice int) error {
  return bus.Dispatch(context.TODO(), product.Create(uuid.New(), name, unitPrice))
}
```

## Stock service

The `Stock` service manages the stock of Products.

### Fill stock of a Product

```go
func fillStock(bus command.Bus, productID uuid.UUID, quantity int) error {
  return bus.Dispatch(context.TODO(), stock.Fill(productID, quantity))
}
```

### Reduce stock of a Product

```go
func reduceStock(bus command.Bus, productID uuid.UUID, quantity int) error {
  return bus.Dispatch(context.TODO(), stock.Reduce(productID, quantity))
}
```

### Reserve stock for an Order

```go
func reserveStock(bus command.Bus, productID, orderID uuid.UUID, quantity int) error {
  return bus.Dispatch(context.TODO(), stock.Reserve(productID, orderID, quantity))
}
```

### Release stock of a (canceled/failed) Order

```go
func releaseStock(bus command.Bus, productID, orderID uuid.UUID) error {
  return bus.Dispatch(context.TODO(), stock.Release(productID, orderID))
}
```

### Execute an Order (claim reserved Stock)

```go
func executeOrder(bus command.Bus, productID, orderID uuid.UUID) error {
  return bus.Dispatch(context.TODO(), stock.Execute(productID, orderID))
}
```

## Order service

### Place an Order (directly / without SAGA)

Here's how to directly create an Order without checking and reserving Stock in
the Stock service:

```go
func placeOrder(bus command.Bus) error {
  cus := order.Customer{
    Name: "Bob",
    Email: "bob@example.com",
  }

  items := []order.Item{
    ProductID: uuid.New(), // use an existing ProductID
    Name: "Example product",
    Quantity: 3,
    UnitPrice: 1495,
  }

  return bus.Dispatch(
    context.TODO(),
    order.Place(uuid.New(), cus, items...),
    dispatch.Synchronous(),
  )
}
```

### Place an Order (with SAGA)

```go
func placeOrder(bus command.Bus, repo aggregate.Repository) error {
  orderID := uuid.New()
  cus := order.Customer{
    Name: "Bob",
    Email: "bob@example.com",
  }
  setup := placeorder.Setup(
    orderID,
    cus,
    placeorder.Item{
      ProductID: uuid.New(), // use a real UUID of a Product here
      Quantity: 3,
    },
    placeorder.Item{
      ProductID: uuid.New(), // use a real UUID of a Product here
      Quantity: 5,
    },
    placeorder.Item{
      ProductID: uuid.New(), // use a real UUID of a Product here
      Quantity: 1,
    },
  )

  exec := saga.NewExecutor(saga.CommandBus(bus), saga.Repository(repo))

  return exec.Execute(context.TODO(), setup)
}
```


### Cancel an Order

```go
func cancelOrder(bus command.Bus, orderID uuid.UUID) error {
  return bus.Dispatch(context.TODO(), order.Cancel(orderID))
}
```
