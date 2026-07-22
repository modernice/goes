package main

import (
	"log"

	"github.com/modernice/goes/eventstoreui"
)

func main() {
	if err := eventstoreui.Main(); err != nil {
		log.Fatal(err)
	}
}
