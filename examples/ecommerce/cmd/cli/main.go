package main

import (
	"ecommerce/cli"
	"ecommerce/cmd"
	"ecommerce/order/mongo"
	"fmt"
	"log"

	"github.com/modernice/goes/aggregate/repository"
)

func main() {
	ereg := cmd.NewEventRegistry()
	ebus := cmd.NewEventBus(ereg, "cli")
	estore := cmd.NewEventStore(ereg, ebus)
	commands := cmd.NewCommandBus(ebus)
	repo := repository.New(estore)
	ctx := cmd.NewContext()

	client, orderDB, err := cmd.InitMongo(ctx, "order")
	if err != nil {
		log.Fatal(err)
	}

	timelines, err := mongo.TimelineRepository(ctx, orderDB)
	if err != nil {
		log.Fatal(err)
	}

	app := cli.New(ebus, commands, repo, timelines)
	cliError := app.Run()

	if err := ebus.Disconnect(ctx); err != nil {
		log.Fatal(fmt.Errorf("disconnect from NATS: %w", err))
	}

	if err := client.Disconnect(ctx); err != nil {
		log.Fatal(fmt.Errorf("disconnect from mongodb: %w", err))
	}

	app.FatalIfErrorf(cliError)
}
