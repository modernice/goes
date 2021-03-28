# ecommerce example

The application consists of 3 decoupled microservices: `Product`, `Stock` and
`Order`.

## Product Service

The `Product` service provides creation of new Products.

### Create a Product

In order to create a Product, dispatch a `product.create` Command:

```go
func createProduct(bus command.Bus, name string, unitPrice int) error {
  return bus.Dispatch(
    context.TODO(),
    product.Create(uuid.New(), name, unitPrice),
    dispatch.Synchronous(), // wait for the Command to be executed
  )
}
```

## Stock Service

The `Stock` service manages the Stock of Products.

### Fill Stock of a Product

```go
func fillStock(bus command.Bus, productID uuid.UUID, quantity int) error {
  return bus.Dispatch(
    context.TODO(),
    stock.Fill(productID, quantity),
    dispatch.Synchronous(),
  )
}
```

### Remove Stock of a Product

```go
func removeStock(bus command.Bus, productID uuid.UUID, quantity int) error {
  return bus.Dispatch(
    context.TODO(),
    stock.Remove(productID, quantity),
    dispatch.Synchronous(),
  )
}
```

### Reserve Stock for an Order

```go
func reserveStock(bus command.Bus, productID, orderID uuid.UUID, quantity int) error {
  return bus.Dispatch(
    context.TODO(),
    stock.Reserve(productID, orderID, quantity),
    dispatch.Synchronous(),
  )
}
```

### Release Stock of a (canceled) Order

```go
func releaseStock(bus command.Bus, productID, orderID uuid.UUID) error {
  return bus.Dispatch(
    context.TODO(),
    stock.Release(productID, orderID),
    dispatch.Synchronous(),
  )
}
```

### Execute an Order (claim reserved Stock)

```go
func executeOrder(bus command.Bus, productID, orderID uuid.UUID) error {
  return bus.Dispatch(
    context.TODO(),
    stock.Execute(productID, orderID),
    dispatch.Synchronous(),
  )
}
```

## Order Service

### Place an Order

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

### Cancel an Order

```go
func cancelOrder(bus command.Bus, orderID uuid.UUID) error {
  return bus.Dispatch(
    context.TODO(),
    order.Cancel(orderID),
    dispatch.Synchronous(),
  )
}
```

## SAGAs

### Place Order SAGA

```go
func placeOrder(bus command.Bus, repo aggregate.Repository) error {
  cus := order.Customer{
    Name: "Bob",
    Email: "bob@example.com",
  }

  orderID := uuid.New()
  items := []placeorder.Item{
    {
      ProductID: uuid.New(), // use an existing ProductID
      Quantity: 3,
    }
  }

  setup := placeorder.Setup(orderID, cus, items...)
  exec := saga.NewExecutor(saga.CommandBus(bus), saga.Repository(repo))

  return exec.Execute(context.TODO(), setup)
}
```
