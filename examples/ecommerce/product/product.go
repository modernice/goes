package product

import (
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/event"
)

const (
	// AggregateName is the name of the Product Aggregate.
	AggregateName = "product"
)

var (
	// ErrAlreadyCreated is returned when trying to create a Product that
	// already was created.
	ErrAlreadyCreated = errors.New("already created")

	// ErrEmptyName is returned when trying to create a Product with an empty name.
	ErrEmptyName = errors.New("empty name")
)

// Product is a Product of the product catalog.
type Product struct {
	*aggregate.Base

	name      string
	unitPrice int
}

// New returns the Product with the given UUID.
func New(id uuid.UUID) *Product {
	return &Product{
		Base: aggregate.New(AggregateName, id),
	}
}

// Name returns the name of the Product.
func (p *Product) Name() string {
	return p.name
}

// UnitPrice returns the unit price in cents.
func (p *Product) UnitPrice() int {
	return p.unitPrice
}

// ApplyEvent applies Events onto the Product to build its state.
func (p *Product) ApplyEvent(evt event.Event) {
	switch evt.Name() {
	case Created:
		p.create(evt)
	}
}

// Create creates the Product with the given name and unit price in cents.
// Create returns ErrEmptyName if the name is an empty string or only
// whitespace or ErrAlreadyCreated if the Product was already created.
func (p *Product) Create(name string, unitPrice int) error {
	if name = strings.TrimSpace(name); name == "" {
		return ErrEmptyName
	}

	if p.name != "" {
		return ErrAlreadyCreated
	}

	aggregate.NextEvent(p, Created, CreatedEvent{
		Name:      name,
		UnitPrice: unitPrice,
	})

	return nil
}

func (p *Product) create(evt event.Event) {
	data := evt.Data().(CreatedEvent)
	p.name = data.Name
	p.unitPrice = data.UnitPrice
}
