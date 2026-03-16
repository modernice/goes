package shop

import (
	"testing"

	"github.com/google/uuid"
)

func TestProductStateTransitions(t *testing.T) {
	product := NewProduct(uuid.New())
	if err := product.Create("Wireless Mouse", 2999, 50); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := product.Rename("Ergonomic Wireless Mouse"); err != nil {
		t.Fatalf("rename: %v", err)
	}
	if err := product.ChangePrice(3499); err != nil {
		t.Fatalf("change price: %v", err)
	}
	if err := product.AdjustStock(-3, "demo sale"); err != nil {
		t.Fatalf("adjust stock: %v", err)
	}

	if product.ProductDTO.Name != "Ergonomic Wireless Mouse" {
		t.Fatalf("unexpected name: %q", product.ProductDTO.Name)
	}
	if product.ProductDTO.Price != 3499 {
		t.Fatalf("unexpected price: %d", product.ProductDTO.Price)
	}
	if product.ProductDTO.Stock != 47 {
		t.Fatalf("unexpected stock: %d", product.ProductDTO.Stock)
	}
}
