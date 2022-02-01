package main

import (
	"fmt"
	"log"
	"time"

	"github.com/modernice/goes/aggregate/repository"
	"github.com/modernice/goes/examples/todo"
	"github.com/modernice/goes/examples/todo/cmd"
	"github.com/modernice/goes/projection/schedule"
)

func main() {
	var setup cmd.Setup

	ctx, cancel := setup.Context()
	defer cancel()

	ebus, estore, ereg, disconnect := setup.Events(ctx)
	defer disconnect()

	cbus, _ := setup.Commands(ereg.Registry, ebus)

	repo := setup.Aggregates(estore)
	lists := repository.Typed(repo, todo.New)

	counter := todo.NewCounter()
	counterErrors, err := counter.Project(ctx, ebus, estore, schedule.Debounce(time.Second))
	if err != nil {
		log.Panic(fmt.Errorf("project counter: %w", err))
	}

	commandErrors := todo.HandleCommands(ctx, cbus, lists)

	cmd.LogErrors(ctx, counterErrors, commandErrors)
}
