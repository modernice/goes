package product

import "github.com/modernice/goes/event"

const (
	// Created means a Product was created.
	Created = "product.created"
)

// CreatedEvent is the Event Data for the Created Event.
type CreatedEvent struct {
	Name      string
	UnitPrice int
}

// RegisterEvents registers Product Events in an Event Registry.
func RegisterEvents(r event.Registry) {
	r.Register(Created, func() event.Data { return CreatedEvent{} })
}
