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
//   - aggregateDeleted ("goes.command.aggregate.deleted") (or a user-provided event, see DeleteEvent())
func PublishEvents(bus event.Bus, store event.Store) HandleOption {
	return func(cfg *handleConfig) {
		cfg.bus = bus
		cfg.store = store
	}
}

// DeleteEvent returns a HandleOption that overrides the deletion event for the
// given aggregate. By default, when the PublishEvents() option is used, a
// "goes.command.aggregate.deleted" event is published when an aggregate is
// deleted from the event store. This option calls the provided makeEvent
// function with a reference to the deleted aggregate to override the published
// event. An empty aggregate name is a wildcard for all aggregates to allow for
// overriding the deletion event for all aggregates.
func DeleteEvent(aggregateName string, makeEvent func(aggregate.Ref) event.Event) HandleOption {
	return func(cfg *handleConfig) {
		cfg.deleteEvents[aggregateName] = makeEvent
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
//   - DeleteAggregateCmd ("goes.command.aggregate.delete")
func Handle(ctx context.Context, bus command.Bus, repo aggregate.Repository, opts ...HandleOption) (<-chan error, error) {
	cfg := handleConfig{deleteEvents: make(map[string]func(aggregate.Ref) event.Of[any])}
	for _, opt := range opts {
		opt(&cfg)
	}

	h := command.NewHandler[any](bus)

	deleteErrors, err := h.Handle(ctx, DeleteAggregateCmd, func(ctx command.Context) error {
		cmd := ctx
		id, name := cmd.Aggregate().Split()
		a := aggregate.New(name, id)

		if err := repo.Fetch(ctx, a); err != nil {
			return fmt.Errorf("fetch aggregate: %w", err)
		}

		if err := repo.Delete(ctx, a); err != nil {
			return fmt.Errorf("delete from repository: %w", err)
		}

		if cfg.bus == nil {
			return nil
		}

		var deletedEvent event.Event
		if makeEvent, ok := cfg.deleteEvents[name]; ok {
			deletedEvent = makeEvent(aggregate.Ref{ID: id, Name: name})
		} else if makeEvent, ok := cfg.deleteEvents[""]; ok {
			deletedEvent = makeEvent(aggregate.Ref{ID: id, Name: name})
		} else {
			deletedEvent = event.New(
				AggregateDeleted,
				AggregateDeletedData{Version: a.AggregateVersion()},
				event.Aggregate(id, name, 0),
			).Any()
		}

		if err := cfg.bus.Publish(ctx, deletedEvent); err != nil {
			return fmt.Errorf("publish %q event: %w", deletedEvent.Name(), err)
		}

		if cfg.store != nil {
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
	bus          event.Bus
	store        event.Store
	deleteEvents map[string]func(aggregate.Ref) event.Event
}
