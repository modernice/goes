package todo

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/event"
)

const ListAggregate = "tour.todo.list"

const (
	ListCreated   = "tour.todo.list.created"
	ItemAdded     = "tour.todo.list.item_added"
	ItemCompleted = "tour.todo.list.item_completed"
)

type (
	ListCreatedEvent   = event.Of[string]
	ItemAddedEvent     = event.Of[string]
	ItemCompletedEvent = event.Of[string]
)

type ListDTO struct {
	ID    uuid.UUID
	Title string
	Items map[string]bool
}

func (dto ListDTO) Created() bool {
	return dto.Title != ""
}

func (dto ListDTO) Print() string {
	out := dto.Title + "\n"
	for _, item := range []string{"Milk", "Eggs"} {
		check := "[ ]"
		if dto.Items[item] {
			check = "[x]"
		}
		out += fmt.Sprintf("  %s %s\n", check, item)
	}
	return out
}

type List struct {
	*aggregate.Base
	ListDTO
}

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
