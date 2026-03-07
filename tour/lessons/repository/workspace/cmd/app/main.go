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

	// TODO: create the in-memory event store.
	// TODO: create a typed product repository.

	id := uuid.New()
	product := shop.NewProduct(id)
	if err := product.Create("Wireless Mouse"); err != nil {
		log.Fatal(err)
	}

	// TODO: save the product.
	// TODO: fetch the product back from the repository.
	// TODO: use the repository to rename the product.
	// TODO: fetch it one more time and print the final name.

	fmt.Println("replace the TODOs to persist and replay the aggregate")
	_, _ = ctx, repository.New
	_, _ = eventstore.New, id
}
