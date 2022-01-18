package builtin

import (
	"context"
	"fmt"

	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/command"
	"github.com/modernice/goes/event"
)

// HandleOption is an option for Handle & MustHandle.
type HandleOption[D any] func(*handleConfig[D])

// PublishEvents returns a HandleOption that configures the command handler to
// publish events over the provided Bus when appropriate. If the optional store
// is non-nil, the events are also inserted into store after publishing.
//
// The following events are published by the handler:
//	- AggregateDeleted ("goes.command.aggregate.deleted")
func PublishEvents[D any](bus event.Bus[D], store event.Store[D]) HandleOption[D] {
	return func(cfg *handleConfig[D]) {
		cfg.bus = bus
		cfg.store = store
	}
}

// MustHandle does the same as Handle, but panic if command registration fails.
func MustHandle[P, D any](ctx context.Context, bus command.Bus[P], repo aggregate.Repository, opts ...HandleOption[D]) <-chan error {
	errs, err := Handle[P, D](ctx, bus, repo, opts...)
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
func Handle[P, D any](ctx context.Context, bus command.Bus[P], repo aggregate.Repository, opts ...HandleOption[D]) (<-chan error, error) {
	var cfg handleConfig[D]
	for _, opt := range opts {
		opt(&cfg)
	}

	h := command.NewHandler(bus)

	deleteErrors, err := h.Handle(ctx, DeleteAggregateCmd, func(ctx command.Context[P]) error {
		cmd := ctx
		id, name := cmd.Aggregate()
		a := aggregate.New(name, id)

		if err := repo.Fetch(ctx, a); err != nil {
			return fmt.Errorf("fetch aggregate: %w", err)
		}

		data := AggregateDeletedData{Version: a.AggregateVersion()}
		tdata, ok := any(data).(D)
		if !ok {
			return fmt.Errorf("publish event: event bus doesn't accept events of type %T", data)
		}

		deletedEvent := event.New(AggregateDeleted, tdata, event.Aggregate[D](id, name, 0))

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

type handleConfig[D any] struct {
	bus   event.Bus[D]
	store event.Store[D]
}
