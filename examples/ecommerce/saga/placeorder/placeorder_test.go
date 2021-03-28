// +build integration

package placeorder_test

import (
	"context"
	"ecommerce/cmd"
	"ecommerce/order"
	"ecommerce/product"
	"ecommerce/saga/placeorder"
	"ecommerce/stock"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/repository"
	"github.com/modernice/goes/command"
	"github.com/modernice/goes/command/cmdbus"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventstore"
	"github.com/modernice/goes/event/eventstore/memstore"
	"github.com/modernice/goes/saga"
)

var (
	exampleCustomer = order.Customer{
		Name:  "Bob",
		Email: "bob@example.com",
	}
)

func TestPlaceOrder(t *testing.T) {
	reg := cmd.NewEventRegistry()
	ebus := cmd.NewEventBus(reg, "test")
	repo, _ := newRepository(ebus)
	products := makeProducts(repo)
	makeStock(repo, products)

	bus, _ := newCommandBus(ebus)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stockErrors, err := stock.HandleCommands(ctx, bus, repo)
	if err != nil {
		t.Fatal(err)
	}

	orderErrors, err := order.HandleCommands(ctx, bus, repo)
	if err != nil {
		t.Fatal(err)
	}

	discardErrors(stockErrors, orderErrors)

	items := makeItems(products)

	orderID := uuid.New()
	setup := placeorder.Setup(orderID, exampleCustomer, items...)

	if err := saga.Execute(context.Background(), setup, saga.CommandBus(bus), saga.Repository(repo)); err != nil {
		t.Fatalf("SAGA failed to execute: %v", err)
	}

	assertOrderPlaced(t, repo, orderID, exampleCustomer, items)
}

func TestPlaceOrder_rollback(t *testing.T) {
	reg := cmd.NewEventRegistry()
	ebus := cmd.NewEventBus(reg, "test")
	repo, _ := newRepository(ebus)
	products := makeProducts(repo)
	makeStock(repo, products)

	bus, _ := newCommandBus(ebus)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stockErrors, err := stock.HandleCommands(ctx, bus, repo)
	if err != nil {
		t.Fatal(err)
	}

	orderErrors, err := order.HandleCommands(ctx, bus, repo)
	if err != nil {
		t.Fatal(err)
	}

	discardErrors(stockErrors, orderErrors)

	orderID := uuid.New()
	setup := placeorder.Setup(orderID, exampleCustomer)

	if err := saga.Execute(
		context.Background(),
		setup,
		saga.CommandBus(bus),
		saga.Repository(repo),
	); err != nil {
		t.Fatalf("SAGA failed to execute: %v", err)
	}

	assertOrderNotPlaced(t, repo, orderID)
}

func newRepository(bus event.Bus) (aggregate.Repository, event.Store) {
	store := eventstore.WithBus(memstore.New(), bus)
	return repository.New(store), store
}

var (
	mockProductMap = map[string]int{
		"Game of Life": 1995,
		"Monopoly":     3990,
	}
)

func makeProducts(repo aggregate.Repository) []*product.Product {
	products := make([]*product.Product, 0, len(mockProductMap))
	for name, price := range mockProductMap {
		p := product.New(uuid.New())
		if err := p.Create(name, price); err != nil {
			panic(fmt.Errorf("create Product: %w", err))
		}
		if err := repo.Save(context.Background(), p); err != nil {
			panic(fmt.Errorf("save Product: %w", err))
		}
		products = append(products, p)
	}

	return products
}

func makeStock(repo aggregate.Repository, products []*product.Product) {
	for _, p := range products {
		s := stock.New(p.AggregateID())
		if err := s.Fill(10); err != nil {
			panic(fmt.Errorf("fill Stock: %w", err))
		}
		if err := repo.Save(context.Background(), s); err != nil {
			panic(fmt.Errorf("save Stock: %w", err))
		}
	}
}

func makeItems(products []*product.Product) []placeorder.Item {
	items := make([]placeorder.Item, len(products))
	for i, p := range products {
		items[i] = placeorder.Item{
			ProductID: p.AggregateID(),
			Quantity:  3,
		}
	}
	return items
}

func newCommandBus(ebus event.Bus) (command.Bus, command.Registry) {
	r := cmd.NewCommandRegistry()
	bus := cmdbus.New(r, ebus)
	return bus, r
}

func assertOrderPlaced(
	t *testing.T,
	repo aggregate.Repository,
	orderID uuid.UUID,
	cus order.Customer,
	items []placeorder.Item,
) {
	o := order.New(orderID)
	if err := repo.Fetch(context.Background(), o); err != nil {
		t.Fatalf("fetch Order: %v", err)
	}

	if o.Customer() != cus {
		t.Errorf("Order customer should be %v; got %v", cus, o.Customer())
	}

	if len(o.Items()) != len(items) {
		t.Fatalf("Order should have %d Items; has %d", len(items), len(o.Items()))
	}

	for _, item := range items {
		var orderItem order.Item
		var ok bool
		for _, oi := range o.Items() {
			if oi.ProductID == item.ProductID {
				orderItem = oi
				ok = true
				break
			}
		}

		if !ok {
			t.Fatalf("Order should have an Item with ProductID=%s", item.ProductID)
		}

		if orderItem.Quantity != item.Quantity {
			t.Fatalf("Order Item %s should have quantity %d; has %d", item.ProductID, item.Quantity, orderItem.Quantity)
		}
	}
}

func assertOrderNotPlaced(t *testing.T, repo aggregate.Repository, orderID uuid.UUID) {
	o := order.New(orderID)
	if err := repo.Fetch(context.Background(), o); err != nil {
		t.Fatal(fmt.Errorf("fetch Order: %w", err))
	}
	if o.Placed() {
		t.Fatalf("Order should not have been placed!")
	}
}

func discardErrors(errs ...<-chan error) {
	for _, errs := range errs {
		errs := errs
		go func() {
			for range errs {
			}
		}()
	}
}
