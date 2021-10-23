package builtin

import (
	"context"
	"fmt"

	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/command"
	"github.com/modernice/goes/event"
)

// Register registers the built-in commands into a command registry.
func Register(r command.Registry) {
	r.Register(DeleteAggregateCmd, func() command.Payload { return DeleteAggregatePayload{} })
}

// HandleOption is an option for Handle & MustHandle.
type HandleOption func(*handleConfig)

// InsertEvents returns a HandleOption that configures the command handler to
// insert published events into the provided event store.
func InsertEvents(store event.Store) HandleOption {
	return func(cfg *handleConfig) {
		cfg.store = store
		cfg.insertEvents = true
	}
}

// MustHandle does the same as Handle, but panic if command registration fails.
func MustHandle(ctx context.Context, bus command.Bus, ebus event.Bus, repo aggregate.Repository, opts ...HandleOption) <-chan error {
	errs, err := Handle(ctx, bus, ebus, repo, opts...)
	if err != nil {
		panic(err)
	}
	return errs
}

// Handle registers command handlers for the built-in commands and returns a
// channel of asynchronous command errors, or a single error if it fails to
// register the commands. When ctx is canceled, command handling stops and the
// returned error channel is closed.
//
// The following commands are handled:
//	- DeleteAggregateCmd ("goes.command.aggregate.delete")
func Handle(ctx context.Context, bus command.Bus, ebus event.Bus, repo aggregate.Repository, opts ...HandleOption) (<-chan error, error) {
	var cfg handleConfig
	for _, opt := range opts {
		opt(&cfg)
	}

	h := command.NewHandler(bus)

	deleteErrors, err := h.Handle(ctx, DeleteAggregateCmd, func(ctx command.Context) error {
		cmd := ctx.Command()
		a := aggregate.New(cmd.AggregateName(), cmd.AggregateID())

		if err := repo.Fetch(ctx, a); err != nil {
			return fmt.Errorf("fetch aggregate: %w", err)
		}

		deletedEvent := a.NextEvent(AggregateDeleted, cmd.AggregateID())

		if err := repo.Delete(ctx, a); err != nil {
			return fmt.Errorf("delete from repository: %w", err)
		}

		if err := ebus.Publish(ctx, deletedEvent); err != nil {
			return fmt.Errorf("publish %q event: %w", deletedEvent.Name(), err)
		}

		if cfg.insertEvents {
			if err := cfg.store.Insert(ctx, deletedEvent); err != nil {
				return fmt.Errorf("insert %q event into event store: %w", deletedEvent.Name(), err)
			}
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("handle %q commands: %w", DeleteAggregateCmd, err)
	}

	return deleteErrors, nil
}

type handleConfig struct {
	// If true, published events are also stored in the event store after being
	// published.
	insertEvents bool
	store        event.Store
}
