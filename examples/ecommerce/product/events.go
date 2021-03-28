package product

import "github.com/modernice/goes/event"

const (
	Created = "product.created"
)

type CreatedEvent struct {
	Name      string
	UnitPrice int
}

func RegisterEvents(r event.Registry) {
	r.Register(Created, func() event.Data { return CreatedEvent{} })
}
