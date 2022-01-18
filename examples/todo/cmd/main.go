package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/modernice/goes/aggregate/repository"
	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/command"
	"github.com/modernice/goes/command/cmdbus"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventbus"
	"github.com/modernice/goes/event/eventstore"
	"github.com/modernice/goes/examples/todo"
	"github.com/modernice/goes/helper/streams"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill, syscall.SIGTERM)
	defer cancel()

	_, ebus, estore := setupEvents()
	_, cbus := setupCommands(ebus)
	repo := repository.New(estore)

	cmdErrors := todo.HandleCommands(ctx, cbus, repo)

	logErrors(ctx, cmdErrors)
}

func setupEvents() (*codec.GobRegistry, event.Bus, event.Store) {
	r := codec.Gob(event.NewRegistry())
	todo.RegisterEvents(r)
	return r, eventbus.New(), eventstore.New()
}

func setupCommands(bus event.Bus) (*codec.GobRegistry, command.Bus) {
	r := codec.Gob(command.NewRegistry())
	todo.RegisterCommands(r)
	return r, cmdbus.New(r, bus)
}

func logErrors(ctx context.Context, errs ...<-chan error) {
	in := streams.FanInContext(ctx, errs...)
	for err := range in {
		log.Println(err)
	}
}
