package main

import (
	"ecommerce/cmd"
	"ecommerce/stock"
	"log"
)

func main() {
	reg := cmd.NewEventRegistry()
	ebus := cmd.NewEventBus(reg, "stock")
	estore := cmd.NewEventStore(reg, ebus)
	cbus := cmd.NewCommandBus(ebus)
	repo := cmd.NewRepository(estore)
	ctx := cmd.NewContext()

	errs, err := stock.HandleCommands(ctx, cbus, repo)
	if err != nil {
		log.Fatal(err)
	}

	<-cmd.LogErrors(errs)
}
