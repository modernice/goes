# 8. The Customer Aggregate

The Customer aggregate demonstrates value objects (addresses) and how to manage collections within an aggregate.

## Define the Customer

Create `customer.go`:

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

const CustomerAggregate = "shop.customer"

// Event names.
const (
	CustomerRegistered = "shop.customer.registered"
	CustomerRenamed    = "shop.customer.renamed"
	AddressAdded       = "shop.customer.address_added"
	AddressRemoved     = "shop.customer.address_removed"
)

// CustomerEvents contains all Customer event names.
var CustomerEvents = [...]string{
	CustomerRegistered,
	CustomerRenamed,
	AddressAdded,
	AddressRemoved,
}

// Event data types.
type CustomerRegisteredData struct {
	Name  string
	Email string
}

type AddressAddedData struct {
	Address Address
}

// Event type aliases.
type (
	CustomerRegisteredEvent = event.Of[CustomerRegisteredData]
	CustomerRenamedEvent    = event.Of[string]
	AddressAddedEvent       = event.Of[AddressAddedData]
	AddressRemovedEvent     = event.Of[uuid.UUID]
)

// Value object — an immutable data structure with an identity.
type Address struct {
	ID      uuid.UUID
	Street  string
	City    string
	ZipCode string
	Country string
}

// CustomerDTO holds the read state of a customer.
type CustomerDTO struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Addresses []Address `json:"addresses"`
}

// Registered reports whether the customer has been initialized.
func (dto CustomerDTO) Registered() bool {
	return dto.Name != ""
}

// Customer is an event-sourced customer.
type Customer struct {
	*aggregate.Base
	CustomerDTO
}

// NewCustomer creates a new Customer aggregate with the given ID.
func NewCustomer(id uuid.UUID) *Customer {
	c := &Customer{
		Base: aggregate.New(CustomerAggregate, id),
		CustomerDTO: CustomerDTO{
			ID:        id,
			Addresses: make([]Address, 0),
		},
	}

	event.ApplyWith(c, c.registered, CustomerRegistered)
	event.ApplyWith(c, c.renamed, CustomerRenamed)
	event.ApplyWith(c, c.addressAdded, AddressAdded)
	event.ApplyWith(c, c.addressRemoved, AddressRemoved)

	return c
}
```

## Business Methods & Appliers

```go
// Register initializes the customer with a name and email.
func (c *Customer) Register(name, email string) error {
	if c.Registered() {
		return fmt.Errorf("customer already registered")
	}
	if name == "" {
		return fmt.Errorf("customer name is required")
	}
	if email == "" {
		return fmt.Errorf("email is required")
	}

	aggregate.Next(c, CustomerRegistered, CustomerRegisteredData{
		Name:  name,
		Email: email,
	})
	return nil
}

func (c *Customer) registered(evt CustomerRegisteredEvent) {
	data := evt.Data()
	c.Name = data.Name
	c.Email = data.Email
}

// Rename changes the customer's name.
func (c *Customer) Rename(name string) error {
	if !c.Registered() {
		return fmt.Errorf("customer not registered")
	}
	if name == "" {
		return fmt.Errorf("customer name is required")
	}
	if name == c.Name {
		return nil
	}

	aggregate.Next(c, CustomerRenamed, name)
	return nil
}

func (c *Customer) renamed(evt CustomerRenamedEvent) {
	c.Name = evt.Data()
}

// AddAddress adds a new address to the customer.
func (c *Customer) AddAddress(addr Address) error {
	if !c.Registered() {
		return fmt.Errorf("customer not registered")
	}
	if addr.Street == "" || addr.City == "" {
		return fmt.Errorf("street and city are required")
	}
	if addr.ID == uuid.Nil {
		addr.ID = uuid.New()
	}

	aggregate.Next(c, AddressAdded, AddressAddedData{Address: addr})
	return nil
}

func (c *Customer) addressAdded(evt AddressAddedEvent) {
	c.Addresses = append(c.Addresses, evt.Data().Address)
}

// RemoveAddress removes an address by its ID.
func (c *Customer) RemoveAddress(addressID uuid.UUID) error {
	if !c.Registered() {
		return fmt.Errorf("customer not registered")
	}

	found := false
	for _, a := range c.Addresses {
		if a.ID == addressID {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("address not found")
	}

	aggregate.Next(c, AddressRemoved, addressID)
	return nil
}

func (c *Customer) addressRemoved(evt AddressRemovedEvent) {
	id := evt.Data()
	for i, a := range c.Addresses {
		if a.ID == id {
			c.Addresses = append(c.Addresses[:i], c.Addresses[i+1:]...)
			return
		}
	}
}
```

## Commands & Registration

```go
const (
	RegisterCustomerCmd = "shop.customer.register"
	RenameCustomerCmd   = "shop.customer.rename"
	AddAddressCmd       = "shop.customer.add_address"
	RemoveAddressCmd    = "shop.customer.remove_address"
)

type RegisterCustomerPayload struct {
	Name  string
	Email string
}

func RegisterCustomerEvents(r codec.Registerer) {
	codec.Register[CustomerRegisteredData](r, CustomerRegistered)
	codec.Register[string](r, CustomerRenamed)
	codec.Register[AddressAddedData](r, AddressAdded)
	codec.Register[uuid.UUID](r, AddressRemoved)
}

func RegisterCustomerCommands(r codec.Registerer) {
	codec.Register[RegisterCustomerPayload](r, RegisterCustomerCmd)
	codec.Register[string](r, RenameCustomerCmd)
	codec.Register[Address](r, AddAddressCmd)
	codec.Register[uuid.UUID](r, RemoveAddressCmd)
}
```

## Command Handlers

```go
// CustomerRepository is the typed repository for customers.
type CustomerRepository = aggregate.TypedRepository[*Customer]

func HandleCustomerCommands(
	ctx context.Context,
	bus command.Bus,
	customers CustomerRepository,
) <-chan error {
	registerErrs := command.MustHandle(ctx, bus, RegisterCustomerCmd, func(ctx command.Ctx[RegisterCustomerPayload]) error {
		return customers.Use(ctx, ctx.AggregateID(), func(c *Customer) error {
			pl := ctx.Payload()
			return c.Register(pl.Name, pl.Email)
		})
	})

	renameErrs := command.MustHandle(ctx, bus, RenameCustomerCmd, func(ctx command.Ctx[string]) error {
		return customers.Use(ctx, ctx.AggregateID(), func(c *Customer) error {
			return c.Rename(ctx.Payload())
		})
	})

	addAddrErrs := command.MustHandle(ctx, bus, AddAddressCmd, func(ctx command.Ctx[Address]) error {
		return customers.Use(ctx, ctx.AggregateID(), func(c *Customer) error {
			return c.AddAddress(ctx.Payload())
		})
	})

	removeAddrErrs := command.MustHandle(ctx, bus, RemoveAddressCmd, func(ctx command.Ctx[uuid.UUID]) error {
		return customers.Use(ctx, ctx.AggregateID(), func(c *Customer) error {
			return c.RemoveAddress(ctx.Payload())
		})
	})

	return streams.FanInAll(registerErrs, renameErrs, addAddrErrs, removeAddrErrs)
}
```

## Wire Into main.go

```go
func main() {
	// ... existing setup ...

	shop.RegisterProductEvents(eventReg)
	shop.RegisterOrderEvents(eventReg)
	shop.RegisterCustomerEvents(eventReg)     // [!code ++]

	shop.RegisterProductCommands(cmdReg)
	shop.RegisterOrderCommands(cmdReg)
	shop.RegisterCustomerCommands(cmdReg)   // [!code ++]

	// ...

	products := repository.Typed(repo, shop.NewProduct)
	orders := repository.Typed(repo, shop.NewOrder)
	customers := repository.Typed(repo, shop.NewCustomer)  // [!code ++]

	// ...

	productErrs := shop.HandleProductCommands(ctx, cbus, products)
	orderErrs := shop.HandleOrderCommands(ctx, cbus, orders, products)
	customerErrs := shop.HandleCustomerCommands(ctx, cbus, customers)  // [!code ++]

	go logErrors(productErrs)
	go logErrors(orderErrs)
	go logErrors(customerErrs)  // [!code ++]

	// ...
}
```

## Value Objects

The `Address` type is a **value object** — it has no independent lifecycle, doesn't have its own event stream, and is always managed through the Customer aggregate. Value objects are identified by their content, not by an ID (though we give addresses an ID for removal purposes).

Compare this to the Order's `LineItem` — another value object. Neither Address nor LineItem is an aggregate.

::: tip When to Make Something an Aggregate vs. a Value Object
If it has its own lifecycle, its own business rules, and needs to be independently addressable — make it an aggregate. If it's just data that belongs to another aggregate — make it a value object.
:::

## Patterns to Notice

1. **Same structure as Product and Order** — `CustomerDTO` with `ID` and `Registered()`, event type aliases, colocated appliers. The pattern is the same across all aggregates.

2. **`Registered()` guards** — `Rename`, `AddAddress`, and `RemoveAddress` all check `!c.Registered()` before proceeding. An unregistered customer can't do anything.

3. **Managing collections** — Adding and removing addresses shows how to manage a list within an aggregate. The `RemoveAddress` applier uses a loop-and-splice pattern.

## Next

We have three aggregates producing events. In the [next chapter](./09-aggregate-splitting), we'll split the Product into multiple focused aggregates that share the same UUID.
