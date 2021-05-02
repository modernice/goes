package stock_test

import (
	"ecommerce/stock"
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestNew(t *testing.T) {
	productID := uuid.New()
	s := stock.New(productID)

	if s.ProductID() != productID {
		t.Errorf("Stock should have ProductID %s; got %s", productID, s.ProductID())
	}

	if s.Quantity() != 0 {
		t.Errorf("Stock should have initial quantity of 0; got %d", s.Quantity())
	}
}

func TestStock_Fill(t *testing.T) {
	s := stock.New(uuid.New())

	if err := s.Fill(20); err != nil {
		t.Fatalf("Fill shouldn't fail; failed with %q", err)
	}

	if err := s.Fill(10); err != nil {
		t.Fatalf("Fill shouldn't fail; failed with %q", err)
	}

	if s.Quantity() != 30 {
		t.Fatalf("Stock should have quantity of %d; got %d", 30, s.Quantity())
	}
}

func TestStock_Reduce(t *testing.T) {
	s := stock.New(uuid.New())
	s.Fill(100)
	if err := s.Reduce(20); err != nil {
		t.Fatalf("Reduce shouldn't fail; failed with %q", err)
	}

	if s.Quantity() != 80 {
		t.Errorf("Stock quantity should be %d; got %d", 80, s.Quantity())
	}
}

func TestStock_Reduce_errInsufficientStock(t *testing.T) {
	s := stock.New(uuid.New())
	s.Fill(100)
	s.Reserve(10, uuid.New())
	if err := s.Reduce(91); !errors.Is(err, stock.ErrInsufficientStock) {
		t.Fatalf("Reduce with a quantity too high should fail with %q; got %q", stock.ErrInsufficientStock, err)
	}
}

func TestStock_Reserve(t *testing.T) {
	s := stock.New(uuid.New())
	if err := s.Fill(100); err != nil {
		t.Fatalf("Fill shouldn't fail; failed with %q", err)
	}

	orderID := uuid.New()
	if err := s.Reserve(10, orderID); err != nil {
		t.Fatalf("Reserve should not fail; failed with %q", err)
	}

	if s.Quantity() != 100 {
		t.Fatalf("Stock should have quantity of 100; got %d", s.Quantity())
	}

	if s.Available() != 90 {
		t.Fatalf("Stock should have available quantity of 90; got %d", s.Available())
	}

	if s.Reserved() != 10 {
		t.Fatalf("Stock should have reserved quantity of 10; got %d", s.Reserved())
	}

	if err := s.Reserve(90, orderID); err != nil {
		t.Fatalf("Reserve should not fail; failed with %q", err)
	}

	if err := s.Fill(10); err != nil {
		t.Fatalf("Fill shouldn't fail; failed with %q", err)
	}

	if err := s.Reserve(11, orderID); !errors.Is(err, stock.ErrStockExceeded) {
		t.Fatalf("Reserve should fail with %q; got %q", stock.ErrStockExceeded, err)
	}
}

func TestStock_Release(t *testing.T) {
	s := stock.New(uuid.New())

	if err := s.Fill(100); err != nil {
		t.Fatalf("failed to fill Stock: %v", err)
	}

	orderID := uuid.New()
	if err := s.Reserve(10, orderID); err != nil {
		t.Fatalf("failed to reserve Stock: %v", err)
	}

	if err := s.Release(orderID); err != nil {
		t.Fatalf("Release shouldn't fail; failed with %q", err)
	}

	if s.Reserved() != 0 {
		t.Errorf("0 quantity should be reserved; got %d", s.Reserved())
	}

	if s.Available() != 100 {
		t.Errorf("100 quantity should be available; got %d", s.Available())
	}
}

func TestStock_Release_errOrderNotFound(t *testing.T) {
	s := stock.New(uuid.New())
	if err := s.Release(uuid.New()); !errors.Is(err, stock.ErrOrderNotFound) {
		t.Fatalf("Release with unknown OrderID should fail with %q; got %q", stock.ErrOrderNotFound, err)
	}
}

func TestStock_Execute(t *testing.T) {
	s := stock.New(uuid.New())
	s.Fill(100)
	orderID := uuid.New()
	s.Reserve(20, orderID)

	if err := s.Execute(orderID); err != nil {
		t.Fatalf("Execute shouldn't fail; failed with %q", err)
	}

	if s.Available() != 80 {
		t.Errorf("available Stock should be %d; got %d", 80, s.Available())
	}

	if s.Reserved() != 0 {
		t.Errorf("reserved Stock should be %d; got %d", 0, s.Reserved())
	}

	if s.Quantity() != 80 {
		t.Errorf("Stock quantity should be %d; got %d", 80, s.Quantity())
	}
}

func TestStock_Execute_errOrderNotFound(t *testing.T) {
	s := stock.New(uuid.New())
	s.Fill(100)
	if err := s.Execute(uuid.New()); !errors.Is(err, stock.ErrOrderNotFound) {
		t.Fatalf("Execute with unknown OrderID should fail with %q; got %d", stock.ErrOrderNotFound, err)
	}
}
