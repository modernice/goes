package order

import "github.com/modernice/goes/event"

const (
	// Placed means an Order was placed.
	Placed = "order.placed"

	// Canceled means an Order was canceled.
	Canceled = "order.canceled"
)

var (
	Events = [...]string{
		Placed,
		Canceled,
	}
)

// PlacedEvent is the Data for the Placed Event.
type PlacedEvent struct {
	Customer Customer
	Items    []Item
}

// CanceledEvent is the Data for the Canceled Event.
type CanceledEvent struct{}

// RegisterEvents registers the Order Events into an Event Registry.
func RegisterEvents(r event.Registry) {
	r.Register(Placed, func() event.Data { return PlacedEvent{} })
	r.Register(Canceled, func() event.Data { return CanceledEvent{} })
}
