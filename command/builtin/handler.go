package builtin

import (
	"context"
	"fmt"

	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/command"
	"github.com/modernice/goes/event"
)

// HandleOption is an option for Handle & MustHandle.
type HandleOption func(*handleConfig)

// PublishEvents returns a HandleOption that configures the command handler to
// publish events over the provided Bus when appropriate. If the optional store
// is non-nil, the events are also inserted into store after publishing.
//
// The following events are published by the handler:
//	- AggregateDeleted ("goes.command.aggregate.deleted")
func PublishEvents(bus event.Bus, store event.Store) HandleOption {
	return func(cfg *handleConfig) {
		cfg.bus = bus
		cfg.store = store
	}
}

// MustHandle does the same as Handle, but panic if command registration fails.
func MustHandle(ctx context.Context, bus command.Bus, repo aggregate.Repository, opts ...HandleOption) <-chan error {
	errs, err := Handle(ctx, bus, repo, opts...)
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
func Handle(ctx context.Context, bus command.Bus, repo aggregate.Repository, opts ...HandleOption) (<-chan error, error) {
	var cfg handleConfig
	for _, opt := range opts {
		opt(&cfg)
	}

	h := command.NewHandler(bus)

	deleteErrors, err := h.Handle(ctx, DeleteAggregateCmd, func(ctx command.Context) error {
		cmd := ctx
		id, name := cmd.Aggregate()
		a := aggregate.New(name, id)

		if err := repo.Fetch(ctx, a); err != nil {
			return fmt.Errorf("fetch aggregate: %w", err)
		}

		deletedEvent := event.New(
			AggregateDeleted,
			AggregateDeletedData{Version: a.AggregateVersion()},
			event.Aggregate(id, name, 0),
		)

		if err := repo.Delete(ctx, a); err != nil {
			return fmt.Errorf("delete from repository: %w", err)
		}

		if cfg.bus != nil {
			if err := cfg.bus.Publish(ctx, deletedEvent); err != nil {
				return fmt.Errorf("publish %q event: %w", deletedEvent.Name(), err)
			}

			if cfg.store != nil {
				if err := cfg.store.Insert(ctx, deletedEvent); err != nil {
					return fmt.Errorf("insert %q event into event store: %w", deletedEvent.Name(), err)
				}
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
	bus   event.Bus
	store event.Store
}
