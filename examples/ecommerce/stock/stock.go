package stock

import (
	"errors"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/event"
)

const (
	// AggregateName is the name of the Stock Aggregate.
	AggregateName = "stock"
)

var (
	// ErrStockExceeded is returned when trying to reserve more Stock than
	// available.
	ErrStockExceeded = errors.New("stock quantity exceeded")

	// ErrInsufficientStock is returned when trying to reduce Stock more than is
	// available.
	ErrInsufficientStock = errors.New("insufficient stock quantity")

	// ErrOrderNotFound is returned when trying to execute/claim Stock for an
	// Order the Stock has no reservation for.
	ErrOrderNotFound = errors.New("order not found")
)

// Stock is the stock of a Product.
type Stock struct {
	*aggregate.Base

	quantity int
	reserved []reservedStock
}

type reservedStock struct {
	OrderID  uuid.UUID
	Quantity int
}

// New returns the Stock for the Product with the given UUID.
func New(id uuid.UUID) *Stock {
	return &Stock{
		Base: aggregate.New(AggregateName, id),
	}
}

// ProductID returns the UUID of the Product.
func (s *Stock) ProductID() uuid.UUID {
	return s.AggregateID()
}

// Quantity returns the total quantity of the Product.
func (s *Stock) Quantity() int {
	return s.quantity
}

// Available returns the stock that is available for order.
func (s *Stock) Available() int {
	return s.Quantity() - s.Reserved()
}

// Reserved returns the stock that is reserved for Orders.
func (s *Stock) Reserved() int {
	var r int
	for _, rs := range s.reserved {
		r += rs.Quantity
	}
	return r
}

// Fill fills the Stock by the given quantity.
func (s *Stock) Fill(quantity int) error {
	aggregate.NextEvent(s, Filled, FilledEvent{
		Quantity: quantity,
	})
	return nil
}

func (s *Stock) fill(evt event.Event) {
	data := evt.Data().(FilledEvent)
	s.quantity += data.Quantity
}

// Reduce removes quantity from the Stock. Reduce returns ErrInsufficientStock
// if quantity is more than the available/unreserved stock.
func (s *Stock) Reduce(quantity int) error {
	if quantity > s.Available() {
		return ErrInsufficientStock
	}
	aggregate.NextEvent(s, Reduced, ReducedEvent{Quantity: quantity})
	return nil
}

func (s *Stock) reduce(evt event.Event) {
	data := evt.Data().(ReducedEvent)
	s.quantity -= data.Quantity
}

// Reserve reserves stock for the given Order.
func (s *Stock) Reserve(q int, orderID uuid.UUID) error {
	if s.Available() < q {
		return ErrStockExceeded
	}
	aggregate.NextEvent(s, Reserved, ReservedEvent{
		Quantity: q,
		OrderID:  orderID,
	})
	return nil
}

func (s *Stock) reserve(evt event.Event) {
	data := evt.Data().(ReservedEvent)
	s.reserved = append(s.reserved, reservedStock{
		OrderID:  data.OrderID,
		Quantity: data.Quantity,
	})
}

// Release releases the reserved Stock for the given Order. Release returns
// ErrOrderNotFound if the Stock has no reservation for that Order.
func (s *Stock) Release(orderID uuid.UUID) error {
	_, _, ok := s.reservedStock(orderID)
	if !ok {
		return ErrOrderNotFound
	}
	aggregate.NextEvent(s, Released, ReleasedEvent{OrderID: orderID})
	return nil
}

func (s *Stock) release(evt event.Event) {
	data := evt.Data().(ReleasedEvent)
	if _, i, ok := s.reservedStock(data.OrderID); ok {
		s.reserved = append(s.reserved[:i], s.reserved[i+1:]...)
	}
}

func (s *Stock) Execute(orderID uuid.UUID) error {
	if _, _, ok := s.reservedStock(orderID); !ok {
		return ErrOrderNotFound
	}
	aggregate.NextEvent(s, Executed, ExecutedEvent{OrderID: orderID})
	return nil
}

func (s *Stock) execute(evt event.Event) {
	data := evt.Data().(ExecutedEvent)
	if rs, i, ok := s.reservedStock(data.OrderID); ok {
		s.reserved = append(s.reserved[:i], s.reserved[i+1:]...)
		s.quantity -= rs.Quantity
	}
}

func (s *Stock) ApplyEvent(evt event.Event) {
	switch evt.Name() {
	case Filled:
		s.fill(evt)
	case Reduced:
		s.reduce(evt)
	case Reserved:
		s.reserve(evt)
	case Released:
		s.release(evt)
	case Executed:
		s.execute(evt)
	}
}

func (s *Stock) reservedStock(orderID uuid.UUID) (rs reservedStock, i int, ok bool) {
	for i, rs := range s.reserved {
		if rs.OrderID == orderID {
			return rs, i, true
		}
	}
	return
}
