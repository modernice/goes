package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/modernice/goes/examples/todo"
	"github.com/modernice/goes/examples/todo/cmd"
	"github.com/modernice/goes/projection/schedule"
)

var intro = `Running "todo" server with the following options:

	TODO_DEBOUNCE: %s
`

func main() {
	debounce := parseDebounce()

	fmt.Printf(intro, debounceText(debounce))

	var setup cmd.Setup

	ctx, cancel := setup.Context()
	defer cancel()

	ebus, estore, ereg, disconnect := setup.Events(ctx, "server")
	defer disconnect()

	cbus, _ := setup.Commands(ereg.Registry, ebus)

	repo := setup.Aggregates(estore)

	counter := todo.NewCounter()
	counterErrors, err := counter.Project(ctx, ebus, estore, schedule.Debounce(debounce))
	if err != nil {
		log.Panic(fmt.Errorf("project counter: %w", err))
	}

	commandErrors := todo.HandleCommands(ctx, cbus, repo)

	cmd.LogErrors(ctx, counterErrors, commandErrors)
}

func parseDebounce() time.Duration {
	if d, err := time.ParseDuration(os.Getenv("TODO_DEBOUNCE")); err == nil {
		return d
	}
	return 0
}

func debounceText(dur time.Duration) string {
	if dur == 0 {
		return fmt.Sprintf("%s (disabled)", dur)
	}
	return dur.String()
}
