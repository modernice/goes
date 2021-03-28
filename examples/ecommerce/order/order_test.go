package order_test

import (
	"ecommerce/order"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
)

var (
	invalidCustomer order.Customer
	exampleCustomer = order.Customer{
		Name:  "Bob",
		Email: "bob@example.com",
	}

	exampleItems = []order.Item{
		{ProductID: uuid.New(), Name: "Lord of the Rings", Quantity: 3, UnitPrice: 1995},
		{ProductID: uuid.New(), Name: "Game of Thrones", Quantity: 2, UnitPrice: 4900},
		{ProductID: uuid.New(), Name: "Monopoly", Quantity: 10, UnitPrice: 2495},
	}
)

func TestNew(t *testing.T) {
	id := uuid.New()
	o := order.New(id)

	if o.AggregateName() != order.AggregateName {
		t.Errorf("Order should have AggregateName %q; is %q", order.AggregateName, o.AggregateName())
	}

	if o.AggregateID() != id {
		t.Errorf("Order should have ID %s; is %s", id, o.AggregateID())
	}

	if l := len(o.Items()); l != 0 {
		t.Errorf("Order should have no Items initially; has %d", l)
	}

	if total := o.Total(); total != 0 {
		t.Errorf("Order should have a Total of 0 cents; is %d cents", total)
	}

	var _ aggregate.Aggregate = o
}

func TestOrder_Place(t *testing.T) {
	o := order.New(uuid.New())

	items := []order.Item{
		{Name: "Lord of the Rings", Quantity: 3, UnitPrice: 1995},
		{Name: "Game of Thrones", Quantity: 2, UnitPrice: 4900},
		{Name: "Monopoly", Quantity: 10, UnitPrice: 2495},
	}

	if err := o.Place(exampleCustomer, items); err != nil {
		t.Fatalf("Place shouldn't fail; failed with %q", err)
	}

	if o.Customer() != exampleCustomer {
		t.Errorf("Order has wrong Customer. want=%v got=%v", exampleCustomer, o.Customer())
	}

	wantTotal := 3*1995 + 2*4900 + 10*2495

	if total := o.Total(); total != wantTotal {
		t.Errorf("Total should return %d cents; got %d cents", wantTotal, total)
	}

	if len(o.AggregateChanges()) != 1 {
		t.Fatalf("Order should have 1 change; got %d", len(o.AggregateChanges()))
	}

	change := o.AggregateChanges()[0]

	if change.Name() != order.Placed {
		t.Errorf("Order should have new %q Event; got %q", order.Placed, change.Name())
	}
}

func TestOrder_Place_invalidCustomer(t *testing.T) {
	o := order.New(uuid.New())
	items := []order.Item{{Name: "Lord of the Rings"}}
	if err := o.Place(invalidCustomer, items); !errors.Is(err, order.ErrInvalidCustomer) {
		t.Fatalf("Place with an empty Customer should fail with %q; got %q", order.ErrInvalidCustomer, err)
	}
}

func TestOrder_Place_errNoItems(t *testing.T) {
	o := order.New(uuid.New())

	if err := o.Place(exampleCustomer, nil); !errors.Is(err, order.ErrNoItems) {
		t.Fatalf("Place with no Items should fail with %q; got %q", order.ErrNoItems, err)
	}

	if len(o.AggregateChanges()) != 0 {
		t.Fatalf("Order should not have changed; has %d changes", len(o.AggregateChanges()))
	}
}

func TestOrder_Place_errAlreadyPlaced(t *testing.T) {
	o := order.New(uuid.New())

	if err := o.Place(exampleCustomer, exampleItems); err != nil {
		t.Fatalf("Place shouldn't fail; failed with %q", err)
	}

	if err := o.Place(exampleCustomer, exampleItems); !errors.Is(err, order.ErrAlreadyPlaced) {
		t.Fatalf("Place should fail with %q; got %q", order.ErrAlreadyPlaced, err)
	}
}

func TestOrder_Cancel(t *testing.T) {
	o := order.New(uuid.New())
	o.Place(exampleCustomer, exampleItems)
	if err := o.Cancel(); err != nil {
		t.Fatalf("Cancel shouldn't fail; failed with %q", err)
	}

	if !o.Canceled() {
		t.Errorf("Order should be canceled!")
	}
}

func TestOrder_Cancel_errNotPlaced(t *testing.T) {
	o := order.New(uuid.New())
	if err := o.Cancel(); !errors.Is(err, order.ErrNotPlaced) {
		t.Fatalf("Cancel should fail with %q; got %q", order.ErrNotPlaced, err)
	}
}

func TestOrder_Cancel_canceled(t *testing.T) {
	o := order.New(uuid.New())
	o.Place(exampleCustomer, exampleItems)
	o.Cancel()
	if err := o.Cancel(); !errors.Is(err, order.ErrCanceled) {
		t.Fatalf("Cancel should fail with %q; got %q", order.ErrCanceled, err)
	}
}
