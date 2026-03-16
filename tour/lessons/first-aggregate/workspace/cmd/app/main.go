package main

import (
	"fmt"
	"log"

	"github.com/google/uuid"
	"tour/firstaggregate/shop"
)

func main() {
	product := shop.NewProduct(uuid.New())
	if err := product.Create("Wireless Mouse", 2999, 50); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("%s: %d cents (%d in stock)\n", product.ProductDTO.Name, product.ProductDTO.Price, product.ProductDTO.Stock)
}
