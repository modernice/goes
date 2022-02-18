package cmd

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate/repository"
	"github.com/modernice/goes/backend/mongo"
	"github.com/modernice/goes/backend/nats"
	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/command"
	"github.com/modernice/goes/command/cmdbus"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/examples/todo"
)

type Setup struct{}

func (s *Setup) Context() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, os.Kill, syscall.SIGTERM)
}

func (s *Setup) Events(ctx context.Context) (_ event.Bus[uuid.UUID], _ event.Store[uuid.UUID], _ *codec.GobRegistry, disconnect func()) {
	log.Printf("Setting up events ...")

	r := codec.Gob(event.NewRegistry())
	todo.RegisterEvents(r)

	bus := nats.NewEventBus(uuid.New, r)
	store := mongo.NewEventStore[uuid.UUID](r)

	return bus, store, r, func() {
		log.Printf("Disconnecting from NATS ...")

		if err := bus.Disconnect(ctx); err != nil {
			log.Panicf("Failed to disconnect from NATS: %v", err)
		}
	}
}

func (s *Setup) Commands(ereg *codec.Registry, ebus event.Bus[uuid.UUID]) (command.Bus[uuid.UUID], *codec.GobRegistry) {
	log.Printf("Setting up commands ...")

	r := codec.Gob(command.NewRegistry())
	todo.RegisterCommands(r)

	cmdbus.RegisterEvents[uuid.UUID](ereg)

	return cmdbus.New(uuid.New, r, ebus), r
}

func (s *Setup) Aggregates(estore event.Store[uuid.UUID]) *repository.Repository[uuid.UUID] {
	log.Printf("Setting up aggregates ...")

	return repository.New(estore)
}
