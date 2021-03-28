package stock

import (
	"errors"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/event"
)

const (
	AggregateName = "stock"
)

var (
	ErrStockExceeded     = errors.New("stock quantity exceeded")
	ErrInsufficientStock = errors.New("insufficient stock quantity")
	ErrOrderNotFound     = errors.New("order not found")
)

type Stock struct {
	aggregate.Aggregate

	quantity int
	reserved []reservedStock
}

type reservedStock struct {
	OrderID  uuid.UUID
	Quantity int
}

func New(id uuid.UUID) *Stock {
	return &Stock{
		Aggregate: aggregate.New(AggregateName, id),
	}
}

func (s *Stock) ProductID() uuid.UUID {
	return s.AggregateID()
}

func (s *Stock) Quantity() int {
	return s.quantity
}

func (s *Stock) Available() int {
	return s.Quantity() - s.Reserved()
}

func (s *Stock) Reserved() int {
	var r int
	for _, rs := range s.reserved {
		r += rs.Quantity
	}
	return r
}

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

func (s *Stock) Remove(quantity int) error {
	if s.quantity < quantity {
		return ErrInsufficientStock
	}
	aggregate.NextEvent(s, Removed, RemovedEvent{Quantity: quantity})
	return nil
}

func (s *Stock) remove(evt event.Event) {
	data := evt.Data().(RemovedEvent)
	s.quantity -= data.Quantity
}

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
	case Removed:
		s.remove(evt)
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
