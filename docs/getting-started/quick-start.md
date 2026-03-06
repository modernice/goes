# Quick Start

This example builds a simple todo list aggregate — you can create a list, add items, and complete them. It saves to an in-memory event store and fetches it back. No external dependencies required.

## The Aggregate

```go
package todo

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/event"
)

// Aggregate name.
const ListAggregate = "todo.list"

// Event names.
const (
	ListCreated   = "todo.list.created"
	ItemAdded     = "todo.list.item_added"
	ItemCompleted = "todo.list.item_completed"
)

// Event data types.
type (
	ListCreatedEvent   = event.Of[string]
	ItemAddedEvent     = event.Of[string]
	ItemCompletedEvent = event.Of[string]
)

// ListDTO holds the read state of a todo list.
// Including the ID makes the DTO self-contained — when you pass it to
// templates, APIs, or other packages, the ID is always available.
type ListDTO struct {
	ID    uuid.UUID       `json:"id"`
	Title string          `json:"title"`
	Items map[string]bool `json:"items"` // item title -> completed
}

// Created reports whether the list has been initialized.
func (dto ListDTO) Created() bool {
	return dto.Title != ""
}

// Print writes the list to stdout.
func (dto ListDTO) Print() {
	fmt.Println(dto.Title)
	for item, done := range dto.Items {
		check := "[ ]"
		if done {
			check = "[x]"
		}
		fmt.Printf("  %s %s\n", check, item)
	}
}

// List is an event-sourced todo list.
type List struct {
	*aggregate.Base
	ListDTO
}

// NewList creates a new todo list aggregate.
func NewList(id uuid.UUID) *List {
	l := &List{
		Base: aggregate.New(ListAggregate, id),
		ListDTO: ListDTO{
			ID:    id,
			Items: make(map[string]bool),
		},
	}

	event.ApplyWith(l, l.created, ListCreated)
	event.ApplyWith(l, l.itemAdded, ItemAdded)
	event.ApplyWith(l, l.itemCompleted, ItemCompleted)

	return l
}

// Create initializes the list with a title.
func (l *List) Create(title string) error {
	if l.Created() {
		return fmt.Errorf("list already created")
	}
	aggregate.Next(l, ListCreated, title)
	return nil
}

func (l *List) created(evt ListCreatedEvent) {
	l.Title = evt.Data()
}

// AddItem adds a new item to the list.
func (l *List) AddItem(title string) error {
	if !l.Created() {
		return fmt.Errorf("list not created")
	}
	if _, exists := l.Items[title]; exists {
		return fmt.Errorf("item %q already exists", title)
	}
	aggregate.Next(l, ItemAdded, title)
	return nil
}

func (l *List) itemAdded(evt ItemAddedEvent) {
	l.Items[evt.Data()] = false
}

// CompleteItem marks an item as done.
func (l *List) CompleteItem(title string) error {
	if !l.Created() {
		return fmt.Errorf("list not created")
	}
	done, exists := l.Items[title]
	if !exists {
		return fmt.Errorf("item %q not found", title)
	}
	if done {
		return fmt.Errorf("item %q already completed", title)
	}
	aggregate.Next(l, ItemCompleted, title)
	return nil
}

func (l *List) itemCompleted(evt ItemCompletedEvent) {
	l.Items[evt.Data()] = true
}
```

## Running It

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/google/uuid"
	"example/todo"
	"github.com/modernice/goes/aggregate/repository"
	"github.com/modernice/goes/event/eventstore"
)

func main() {
	ctx := context.Background()

	// Set up the event store and repository.
	store := eventstore.New()
	lists := repository.Typed(repository.New(store), todo.NewList)

	// Create a list and add some items.
	id := uuid.New()
	list := todo.NewList(id)

	if err := list.Create("Groceries"); err != nil {
		log.Fatal(err)
	}
	list.AddItem("Milk")
	list.AddItem("Eggs")
	list.CompleteItem("Milk")

	// Save — uncommitted events are inserted into the store.
	if err := lists.Save(ctx, list); err != nil {
		log.Fatal(err)
	}

	// Fetch — events are replayed to reconstruct state.
	fetched, err := lists.Fetch(ctx, id)
	if err != nil {
		log.Fatal(err)
	}

	fetched.Print()
	// Output:
	// Groceries
	//   [x] Milk
	//   [ ] Eggs
}
```

## What Just Happened?

1. **Defined an aggregate** — `List` embeds `*aggregate.Base` (which provides event-sourcing mechanics) and a `ListDTO` that holds the actual state.
2. **Registered event handlers** — `event.ApplyWith` connects event names to handler methods. The handler methods are unexported and only mutate state — no validation, no side effects.
3. **Raised events** — Domain methods like `Create` and `AddItem` validate the operation, then call `aggregate.Next` to record what happened. The event is immediately applied to the aggregate.
4. **Saved and fetched** — The repository inserted the uncommitted events into the store. On fetch, it replayed them through the handlers to reconstruct state.

## Next Steps

Ready to build something real? The [tutorial](/tutorial/) walks you through building a complete e-commerce application with products, orders, customers, and projections.
