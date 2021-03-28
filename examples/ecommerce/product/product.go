package product

import (
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/event"
)

const (
	AggregateName = "product"
)

var (
	ErrAlreadyCreated = errors.New("already created")
	ErrEmptyName      = errors.New("empty name")
)

type Product struct {
	aggregate.Aggregate

	name      string
	unitPrice int
}

func New(id uuid.UUID) *Product {
	return &Product{
		Aggregate: aggregate.New(AggregateName, id),
	}
}

func (p *Product) Name() string {
	return p.name
}

func (p *Product) UnitPrice() int {
	return p.unitPrice
}

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

func (p *Product) ApplyEvent(evt event.Event) {
	switch evt.Name() {
	case Created:
		p.create(evt)
	}
}
