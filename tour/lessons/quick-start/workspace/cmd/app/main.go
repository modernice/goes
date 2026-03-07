package main

import (
	"context"
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate/repository"
	"github.com/modernice/goes/event/eventstore"
	"tour/quickstart/todo"
)

func main() {
	ctx := context.Background()
	store := eventstore.New()
	lists := repository.Typed(repository.New(store), todo.NewList)

	id := uuid.New()
	list := todo.NewList(id)

	if err := list.Create("Groceries"); err != nil {
		log.Fatal(err)
	}
	if err := list.AddItem("Milk"); err != nil {
		log.Fatal(err)
	}
	if err := list.AddItem("Eggs"); err != nil {
		log.Fatal(err)
	}
	if err := list.CompleteItem("Milk"); err != nil {
		log.Fatal(err)
	}

	if err := lists.Save(ctx, list); err != nil {
		log.Fatal(err)
	}

	fetched, err := lists.Fetch(ctx, id)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Print(fetched.Print())
}
