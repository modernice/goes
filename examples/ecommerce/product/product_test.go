package product_test

import (
	"ecommerce/product"
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestProduct_Create(t *testing.T) {
	p := product.New(uuid.New())
	if err := p.Create("Game of Life", 1500); err != nil {
		t.Fatalf("Create shouldn't fail; failed with %q", err)
	}

	if p.Name() != "Game of Life" {
		t.Errorf("Product name should be %q; got %q", "Game of Life", p.Name())
	}

	if p.UnitPrice() != 1500 {
		t.Errorf("Unit price should be %d cents; got %d cents", 1500, p.UnitPrice())
	}
}

func TestProduct_Create_emptyName(t *testing.T) {
	p := product.New(uuid.New())
	if err := p.Create("   ", 1000); !errors.Is(err, product.ErrEmptyName) {
		t.Fatalf("Create with an empty name should fail with %q; got %q", product.ErrEmptyName, err)
	}
}

func TestProduct_Create_errAlreadyCreated(t *testing.T) {
	p := product.New(uuid.New())
	p.Create("Game of Life", 1500)
	if err := p.Create("Game of Life", 1500); !errors.Is(err, product.ErrAlreadyCreated) {
		t.Fatalf("Create should fail with %q; got %q", product.ErrAlreadyCreated, err)
	}
}
