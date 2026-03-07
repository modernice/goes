package todo

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate/repository"
	"github.com/modernice/goes/event/eventstore"
)

func TestQuickStartListLifecycle(t *testing.T) {
	ctx := context.Background()
	store := eventstore.New()
	lists := repository.Typed(repository.New(store), NewList)

	id := uuid.New()
	list := NewList(id)

	if err := list.Create("Groceries"); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := list.AddItem("Milk"); err != nil {
		t.Fatalf("add milk: %v", err)
	}
	if err := list.AddItem("Eggs"); err != nil {
		t.Fatalf("add eggs: %v", err)
	}
	if err := list.CompleteItem("Milk"); err != nil {
		t.Fatalf("complete milk: %v", err)
	}
	if err := lists.Save(ctx, list); err != nil {
		t.Fatalf("save: %v", err)
	}

	fetched, err := lists.Fetch(ctx, id)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}

	if fetched.Title != "Groceries" {
		t.Fatalf("unexpected title: %q", fetched.Title)
	}
	if !fetched.Items["Milk"] {
		t.Fatalf("expected Milk to be completed")
	}
	if fetched.Items["Eggs"] {
		t.Fatalf("expected Eggs to be incomplete")
	}
}
