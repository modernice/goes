package shop

import (
	"testing"

	"github.com/google/uuid"
)

func TestProductCreateAppliesState(t *testing.T) {
	product := NewProduct(uuid.New())
	if err := product.Create("Wireless Mouse", 2999, 50); err != nil {
		t.Fatalf("create: %v", err)
	}

	if !product.Created() {
		t.Fatalf("expected product to be created")
	}
	if product.ProductDTO.Name != "Wireless Mouse" || product.ProductDTO.Price != 2999 || product.ProductDTO.Stock != 50 {
		t.Fatalf("unexpected product state: %+v", product.ProductDTO)
	}
}
