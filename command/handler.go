package command

import (
	"context"
	"fmt"

	"github.com/modernice/goes/command/done"
)

// A Handler handles Commands.
type Handler struct {
	bus Bus
}

// Handle subscribes to Commands with the given name and executes them by calling fn
// with the received Command.
func Handle(
	ctx context.Context,
	bus Bus,
	name string,
	fn func(context.Context, Command) error,
) (<-chan error, error) {
	return NewHandler(bus).On(ctx, name, fn)
}

// NewHandler returns a Handler that can be used to subscribe to and handle Commands.
func NewHandler(bus Bus) *Handler {
	return &Handler{bus}
}

// On subscribes to Commands with the provided name and handles them with the provided fn.
func (h *Handler) On(
	ctx context.Context,
	name string,
	fn func(context.Context, Command) error,
) (<-chan error, error) {
	commands, errs, err := h.bus.Subscribe(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("command bus: %w", err)
	}

	out := make(chan error)

	go func() {
		defer close(out)
		for {
			if errs == nil && commands == nil {
				return
			}

			select {
			case err, ok := <-errs:
				if !ok {
					errs = nil
					break
				}
				out <- err
			case ctx, ok := <-commands:
				if !ok {
					commands = nil
					break
				}
				if err := h.handle(ctx, fn); err != nil {
					out <- err
				}
			}
		}
	}()

	return out, nil
}

func (h *Handler) handle(ctx Context, fn func(context.Context, Command) error) error {
	handleError := fn(ctx, ctx.Command())

	if err := ctx.MarkDone(ctx, done.WithError(handleError)); err != nil {
		return fmt.Errorf("mark as done: %w", err)
	}

	return handleError
}
