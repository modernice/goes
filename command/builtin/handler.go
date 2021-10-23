package builtin

import (
	"context"
	"fmt"

	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/command"
)

// Register registers the built-in commands into a command registry.
func Register(r command.Registry) {
	r.Register(DeleteAggregateCmd, func() command.Payload { return DeleteAggregatePayload{} })
}

// MustHandle does the same as Handle, but panic if command registration fails.
func MustHandle(ctx context.Context, bus command.Bus, repo aggregate.Repository) <-chan error {
	errs, err := Handle(ctx, bus, repo)
	if err != nil {
		panic(err)
	}
	return errs
}

// Handle registers command handlers for the built-in commands and returns a
// channel of asynchronous command errors, or a single error if it fails to
// register the commands. When ctx is canceled, command handling stops and the
// returned error channel is closed.
func Handle(ctx context.Context, bus command.Bus, repo aggregate.Repository) (<-chan error, error) {
	h := command.NewHandler(bus)

	deleteErrors, err := h.Handle(ctx, DeleteAggregateCmd, func(ctx command.Context) error {
		cmd := ctx.Command()
		return repo.Delete(ctx, aggregate.New(cmd.AggregateName(), cmd.AggregateID()))
	})
	if err != nil {
		return nil, fmt.Errorf("handle %q commands: %w", DeleteAggregateCmd, err)
	}

	return deleteErrors, nil
}
