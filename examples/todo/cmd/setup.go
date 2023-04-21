package cmd

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/modernice/goes/aggregate/repository"
	"github.com/modernice/goes/backend/mongo"
	"github.com/modernice/goes/backend/nats"
	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/command"
	"github.com/modernice/goes/command/cmdbus"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventstore"
	"github.com/modernice/goes/examples/todo"
)

// Setup is a type that provides methods to set up the necessary components for
// running the "todo" application. It includes methods to obtain a context with
// cancellation, set up event buses and stores, register commands, and create a
// repository for aggregates.
type Setup struct{}

// Context returns a context.Context that is cancelled when the program receives
// an interrupt signal (os.Interrupt, os.Kill, syscall.SIGTERM).
func (s *Setup) Context() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, os.Kill, syscall.SIGTERM)
}

// Events returns an event bus, an event store, a codec registry and a
// disconnect function. The event bus is an instance of
// [event.Bus](https://pkg.go.dev/github.com/modernice/goes/event#Bus) that can
// be used to publish events. The event store is an instance of
// [event.Store](https://pkg.go.dev/github.com/modernice/goes/event/eventstore#Store)
// that can be used to store and retrieve events. The codec registry is an
// instance of
// [codec.Registry](https://pkg.go.dev/github.com/modernice/goes/codec#Registry)
// that can be used to marshal and unmarshal events. The disconnect function
// should be called when the application shuts down to disconnect from the event
// bus.
func (s *Setup) Events(ctx context.Context, serviceName string) (_ event.Bus, _ event.Store, _ *codec.Registry, disconnect func()) {
	log.Printf("Setting up events ...")

	r := event.NewRegistry()
	todo.RegisterEvents(r)

	bus, disconnect := s.EventBus(ctx, r, serviceName)
	store := eventstore.WithBus(mongo.NewEventStore(r), bus)

	return bus, store, r, disconnect
}

// EventBus returns a new NATS
// [event.Bus](https://pkg.go.dev/github.com/modernice/goes/event#Bus) that uses
// the given codec.Encoding. The returned function can be used to disconnect
// from the NATS server.
func (s *Setup) EventBus(ctx context.Context, enc codec.Encoding, serviceName string) (_ event.Bus, disconnect func()) {
	bus := nats.NewEventBus(enc)

	return bus, func() {
		log.Printf("Disconnecting from NATS ...")

		if err := bus.Disconnect(ctx); err != nil {
			log.Panicf("Failed to disconnect from NATS: %v", err)
		}
	}
}

// Commands returns a command.Bus and a *codec.Registry for registering and
// handling commands for the todo application. It takes a *codec.Registry and an
// event.Bus as arguments, used to register events for commands.
func (s *Setup) Commands(ereg *codec.Registry, ebus event.Bus) (command.Bus, *codec.Registry) {
	log.Printf("Setting up commands ...")

	r := command.NewRegistry()
	todo.RegisterCommands(r)

	cmdbus.RegisterEvents(ereg)

	return cmdbus.New[int](r, ebus), r
}

// Aggregates returns a *repository.Repository for managing aggregates using the
// given event.Store.
func (s *Setup) Aggregates(estore event.Store) *repository.Repository {
	log.Printf("Setting up aggregates ...")

	return repository.New(estore)
}
