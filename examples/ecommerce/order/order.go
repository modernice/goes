package order

import (
	"errors"
	"net/mail"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/event"
)

const (
	// AggregateName is the name of the Order Aggregate.
	AggregateName = "order"
)

var (
	// ErrInvalidCustomer is returned when a Customer is invalid.
	ErrInvalidCustomer = errors.New("invalid customer data")

	// ErrNoItems is returned when an Order is placed without any Items.
	ErrNoItems = errors.New("cannot place order without items")

	// ErrAlreadyPlaced is returned when trying to place an Order that was
	// placed already.
	ErrAlreadyPlaced = errors.New("order has already been placed")

	// ErrNotPlaced is returned when trying to act on an Order that hasn't been
	// placed yet.
	ErrNotPlaced = errors.New("order has not been placed yet")

	// ErrCanceled is returned when trying to act on a canceled Order.
	ErrCanceled = errors.New("order was canceled")
)

// An Order is an order in the ecommerce app.
type Order struct {
	// Embedding *aggregate.Base is not needed if aggregate.Aggregate is
	// implemented manually. But who wants to do that?
	//
	// Alternatively embed aggregate.Aggregate (but be aware that marshalling of
	// interface types may cause problems)
	*aggregate.Base

	customer Customer
	items    []Item

	canceledAt time.Time
}

// Customer is the customer of an Order.
type Customer struct {
	Name  string
	Email string
}

// Item is an item of an order.
type Item struct {
	ProductID uuid.UUID
	Name      string
	Quantity  int
	UnitPrice int
}

// New returns a new Order.
func New(id uuid.UUID) *Order {
	return &Order{
		Base: aggregate.New(AggregateName, id),

		// If you embedded aggregate.Aggregate instead of *aggregate.Base:
		// Aggregate: aggregate.New(AggregateName, id),
	}
}

// Customer returns the Customer of the Order.
func (o *Order) Customer() Customer {
	return o.customer
}

// Items returns the ordered Items.
func (o *Order) Items() []Item {
	return o.items
}

// Total returns the price of the Order in cents.
func (o *Order) Total() int {
	var total int
	for _, item := range o.items {
		total += item.Quantity * item.UnitPrice
	}
	return total
}

// Canceled returns whether the Order was cancelled.
func (o *Order) Canceled() bool {
	return !o.canceledAt.IsZero()
}

// ApplyEvent implements aggregate.Aggregate.
func (o *Order) ApplyEvent(evt event.Event) {
	switch evt.Name() {
	case Placed:
		o.place(evt)
	case Canceled:
		o.cancel(evt)
	}
}

// Place places the Order for the given Customer.
//
// Place returns ErrAlreadyPlaced if the Order has been placed already. If no
// Items are provided, Place returns ErrNoItems.
func (o *Order) Place(cus Customer, items []Item) error {
	if o.Placed() {
		return ErrAlreadyPlaced
	}

	if err := cus.validate(); err != nil {
		return err
	}

	if len(items) == 0 {
		return ErrNoItems
	}

	// This is the important part:
	// We use aggregate.NextEvent to create and apply the "order.placed" Event.
	// aggregate.NextEvent calls o.ApplyEvent with the created Event, which in
	// turn calls o.place with that Event. o.place is finally applies the Event
	// and updates the fields of the Order.
	aggregate.NextEvent(o, Placed, PlacedEvent{
		Customer: cus,
		Items:    items,
	})

	return nil
}

// place applies the "order.placed" Event.
func (o *Order) place(evt event.Event) {
	data := evt.Data().(PlacedEvent)
	o.customer = data.Customer
	o.items = data.Items
}

// Placed returns whether the Order has been placed already.
func (o *Order) Placed() bool {
	// o.Place fails when no Items are provided, so we can say that an Order has
	// been placed if it has Items.
	return len(o.items) > 0
}

// Cancel canels the Order. If the Order hasn't been placed yet, Cancel returns
// ErrNotPlaced. If the Order was canceled already, Cancel returns ErrCanceled.
func (o *Order) Cancel() error {
	if !o.Placed() {
		return ErrNotPlaced
	}
	if o.Canceled() {
		return ErrCanceled
	}
	aggregate.NextEvent(o, Canceled, CanceledEvent{})
	return nil
}

// cancel applies the "order.canceled" Event.
func (o *Order) cancel(evt event.Event) {
	o.canceledAt = evt.Time()
}

func (c Customer) validate() error {
	if c.Name == "" {
		return ErrInvalidCustomer
	}
	if _, err := mail.ParseAddress(c.Email); err != nil {
		return ErrInvalidCustomer
	}
	return nil
}
