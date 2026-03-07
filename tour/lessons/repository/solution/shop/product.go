package shop

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/event"
)

const ProductAggregate = "tour.shop.product"

const (
	ProductCreated = "tour.shop.product.created"
	ProductRenamed = "tour.shop.product.renamed"
)

type (
	ProductCreatedEvent = event.Of[string]
	ProductRenamedEvent = event.Of[string]
)

type ProductDTO struct {
	ID   uuid.UUID
	Name string
}

func (dto ProductDTO) Created() bool {
	return dto.Name != ""
}

type Product struct {
	*aggregate.Base
	ProductDTO
}

func NewProduct(id uuid.UUID) *Product {
	p := &Product{
		Base:       aggregate.New(ProductAggregate, id),
		ProductDTO: ProductDTO{ID: id},
	}

	event.ApplyWith(p, p.created, ProductCreated)
	event.ApplyWith(p, p.renamed, ProductRenamed)

	return p
}

func (p *Product) Create(name string) error {
	if p.Created() {
		return fmt.Errorf("product already created")
	}
	if name == "" {
		return fmt.Errorf("product name is required")
	}

	aggregate.Next(p, ProductCreated, name)
	return nil
}

func (p *Product) Rename(name string) error {
	if !p.Created() {
		return fmt.Errorf("product not created")
	}
	if name == "" {
		return fmt.Errorf("product name is required")
	}
	if name == p.ProductDTO.Name {
		return nil
	}

	aggregate.Next(p, ProductRenamed, name)
	return nil
}

func (p *Product) created(evt ProductCreatedEvent) {
	p.ProductDTO.Name = evt.Data()
}

func (p *Product) renamed(evt ProductRenamedEvent) {
	p.ProductDTO.Name = evt.Data()
}
