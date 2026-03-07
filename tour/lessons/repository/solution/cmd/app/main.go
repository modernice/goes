package main

import (
	"context"
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate/repository"
	"github.com/modernice/goes/event/eventstore"
	"tour/repository/shop"
)

func main() {
	ctx := context.Background()
	store := eventstore.New()
	products := repository.Typed(repository.New(store), shop.NewProduct)

	id := uuid.New()
	product := shop.NewProduct(id)
	if err := product.Create("Wireless Mouse"); err != nil {
		log.Fatal(err)
	}
	if err := products.Save(ctx, product); err != nil {
		log.Fatal(err)
	}

	fetched, err := products.Fetch(ctx, id)
	if err != nil {
		log.Fatal(err)
	}
	if err := products.Use(ctx, id, func(p *shop.Product) error {
		return p.Rename("Ergonomic Wireless Mouse")
	}); err != nil {
		log.Fatal(err)
	}

	renamed, err := products.Fetch(ctx, fetched.ProductDTO.ID)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(renamed.ProductDTO.Name)
}
