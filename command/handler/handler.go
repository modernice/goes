package handler

import (
	"context"
	"fmt"

	"github.com/modernice/goes/command"
	"github.com/modernice/goes/command/done"
)

// A Handler handles Commands.
type Handler struct {
	bus command.Bus
}

// New returns a new Command Handler. The Handler subscribes to Commands using
// the provided Bus.
func New(bus command.Bus) *Handler {
	return &Handler{
		bus: bus,
	}
}

// On subscribes to Commands with the given name and executes them by calling fn
// with the received Command.
func (h *Handler) On(
	ctx context.Context,
	name string,
	fn func(context.Context, command.Command) error,
) (<-chan error, error) {
	commands, errs, err := h.bus.Subscribe(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("command bus: %w", err)
	}

	out := make(chan error)

	go func() {
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				return
			case err, ok := <-errs:
				if !ok {
					errs = nil
					break
				}
				select {
				case <-ctx.Done():
				case out <- err:
				}
			case ctx, ok := <-commands:
				if !ok {
					return
				}
				if err := h.handle(ctx, fn); err != nil {
					select {
					case <-ctx.Done():
					case out <- err:
					}
				}
			}
		}
	}()

	return out, nil
}

func (h *Handler) handle(ctx command.Context, fn func(context.Context, command.Command) error) error {
	handleError := fn(ctx, ctx.Command())

	if err := ctx.MarkDone(ctx, done.WithError(handleError)); err != nil {
		return fmt.Errorf("mark as done: %w", err)
	}

	return handleError
}
