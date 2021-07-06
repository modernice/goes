package main

import (
	"ecommerce/cmd"
	"ecommerce/order"
	mongorepo "ecommerce/order/mongo"
	"fmt"
	"log"

	"github.com/logrusorgru/aurora"
)

func main() {
	reg := cmd.NewEventRegistry()
	ebus := cmd.NewEventBus(reg, "order")
	estore := cmd.NewEventStore(reg, ebus)
	cbus := cmd.NewCommandBus(reg, ebus)
	repo := cmd.NewRepository(estore)
	ctx := cmd.NewContext()

	cmdErrors, err := order.HandleCommands(ctx, cbus, repo)
	if err != nil {
		log.Fatal(err)
	}

	client, db, err := cmd.InitMongo(ctx, "order")
	if err != nil {
		log.Fatal(fmt.Errorf("init mongo: %w", err))
	}

	timelineRepo, err := mongorepo.TimelineRepository(ctx, db)
	if err != nil {
		log.Fatal(fmt.Errorf("make Timeline Repository: %w", err))
	}

	timelineProjector := order.NewTimelineProjector(ebus, estore, timelineRepo)

	timelineErrors, err := timelineProjector.Run(ctx)
	if err != nil {
		log.Fatalf("start TimelineProjector: %v", err)
	}

	log.Println(aurora.Green("Service started."))

	<-cmd.LogErrors(cmdErrors, timelineErrors)

	log.Println(aurora.Yellow("Shutting down..."))

	ctx = cmd.NewContext()

	if err := ebus.Disconnect(ctx); err != nil {
		log.Fatal(err)
	}

	if err := client.Disconnect(ctx); err != nil {
		log.Fatal(fmt.Errorf("disconnect from mongodb: %w", err))
	}

	log.Println(aurora.Yellow("Service stopped."))
}
