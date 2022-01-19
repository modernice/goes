package main

import (
	"github.com/modernice/goes/aggregate/repository"
	"github.com/modernice/goes/examples/todo"
	"github.com/modernice/goes/examples/todo/cmd"
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

	errs := todo.HandleCommands(ctx, cbus, lists)

	cmd.LogErrors(ctx, errs)
}
