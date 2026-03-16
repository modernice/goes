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
	PriceChanged   = "tour.shop.product.price_changed"
	StockAdjusted  = "tour.shop.product.stock_adjusted"
)

type ProductCreatedData struct {
	Name  string
	Price int
	Stock int
}

type StockAdjustedData struct {
	Quantity int
	Reason   string
}

type (
	ProductCreatedEvent = event.Of[ProductCreatedData]
	ProductRenamedEvent = event.Of[string]
	PriceChangedEvent   = event.Of[int]
	StockAdjustedEvent  = event.Of[StockAdjustedData]
)

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
		Base:       aggregate.New(ProductAggregate, id),
		ProductDTO: ProductDTO{ID: id},
	}

	event.ApplyWith(p, p.created, ProductCreated)
	event.ApplyWith(p, p.renamed, ProductRenamed)
	event.ApplyWith(p, p.priceChanged, PriceChanged)
	event.ApplyWith(p, p.stockAdjusted, StockAdjusted)

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

func (p *Product) renamed(evt ProductRenamedEvent) {
	p.ProductDTO.Name = evt.Data()
}

func (p *Product) ChangePrice(price int) error {
	if !p.Created() {
		return fmt.Errorf("product not created")
	}
	if price <= 0 {
		return fmt.Errorf("price must be positive")
	}
	if price == p.ProductDTO.Price {
		return nil
	}

	aggregate.Next(p, PriceChanged, price)
	return nil
}

func (p *Product) priceChanged(evt PriceChangedEvent) {
	p.ProductDTO.Price = evt.Data()
}

func (p *Product) AdjustStock(quantity int, reason string) error {
	if !p.Created() {
		return fmt.Errorf("product not created")
	}
	if p.ProductDTO.Stock+quantity < 0 {
		return fmt.Errorf("insufficient stock")
	}

	aggregate.Next(p, StockAdjusted, StockAdjustedData{Quantity: quantity, Reason: reason})
	return nil
}

func (p *Product) stockAdjusted(evt StockAdjustedEvent) {
	p.ProductDTO.Stock += evt.Data().Quantity
}
