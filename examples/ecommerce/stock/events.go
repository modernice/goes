package stock

import (
	"github.com/google/uuid"
	"github.com/modernice/goes/event"
)

const (
	// Filled means Stock has been filled.
	Filled = "stock.filled"

	// Reduced means Stock has been reduced.
	Reduced = "stock.removed"

	// Reserved means Stock has been reserved for an Order.
	Reserved = "stock.reserved"

	// Released means Stock for an Order has been released.
	Released = "stock.released"

	// Executed means an Order was executed and the reserved Stock for that
	// Order removed from the Stock.
	Executed = "stock.executed"
)

// FilledEvent is the Event Data for the Filled Event.
type FilledEvent struct {
	Quantity int
}

// ReducedEvent is the Event Data for the Reduced Event.
type ReducedEvent struct {
	Quantity int
}

// ReservedEvent is the Event Data for the ReservedEvent.
type ReservedEvent struct {
	Quantity int
	OrderID  uuid.UUID
}

// ReleasedEvent is the Event Data for the Released Event.
type ReleasedEvent struct {
	OrderID uuid.UUID
}

// ExecutedEvent is the Event Data for the Executed Event.
type ExecutedEvent struct {
	OrderID uuid.UUID
}

// RegisterEvents registers the Stock Events into an Event Registry.
func RegisterEvents(r event.Registry) {
	r.Register(Filled, func() event.Data { return FilledEvent{} })
	r.Register(Reduced, func() event.Data { return ReducedEvent{} })
	r.Register(Reserved, func() event.Data { return ReservedEvent{} })
	r.Register(Released, func() event.Data { return ReleasedEvent{} })
	r.Register(Executed, func() event.Data { return ExecutedEvent{} })
}
