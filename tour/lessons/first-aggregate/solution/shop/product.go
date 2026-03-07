package shop

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/event"
)

const ProductAggregate = "tour.shop.product"

const ProductCreated = "tour.shop.product.created"

type ProductCreatedData struct {
	Name  string
	Price int
	Stock int
}

type ProductCreatedEvent = event.Of[ProductCreatedData]

type ProductDTO struct {
	ID    uuid.UUID
	Name  string
	Price int
	Stock int
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
		Base: aggregate.New(ProductAggregate, id),
		ProductDTO: ProductDTO{
			ID: id,
		},
	}

	event.ApplyWith(p, p.created, ProductCreated)

	return p
}

func (p *Product) Create(name string, price, stock int) error {
	if p.Created() {
		return fmt.Errorf("product already created")
	}
	if name == "" {
		return fmt.Errorf("product name is required")
	}
	if price <= 0 {
		return fmt.Errorf("price must be positive")
	}
	if stock < 0 {
		return fmt.Errorf("stock cannot be negative")
	}

	aggregate.Next(p, ProductCreated, ProductCreatedData{Name: name, Price: price, Stock: stock})
	return nil
}

func (p *Product) created(evt ProductCreatedEvent) {
	data := evt.Data()
	p.ProductDTO.Name = data.Name
	p.ProductDTO.Price = data.Price
	p.ProductDTO.Stock = data.Stock
}
