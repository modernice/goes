package handler

import (
	"context"
	"fmt"
	"sync"

	"github.com/modernice/goes/command"
	"github.com/modernice/goes/command/done"
	"github.com/modernice/goes/internal/errbus"
)

// TODO: add docs
type Handler struct {
	bus  command.Bus
	errs *errbus.Bus
}

// New returns a new Handler that handles Commands using the provided Bus.
func New(bus command.Bus) *Handler {
	return &Handler{
		bus:  bus,
		errs: errbus.New(),
	}
}

// On subscribes to Commands with the given name and executes by calling fn with
// the received Command.
func (h *Handler) On(
	ctx context.Context,
	name string,
	fn func(context.Context, command.Command) error,
) error {
	commands, errs, err := h.bus.Subscribe(ctx, name)
	if err != nil {
		return fmt.Errorf("bus: %w", err)
	}

	errc := make(chan error)
	go func() {
		defer close(errc)
		for {
			select {
			case err, ok := <-errs:
				if !ok {
					errs = nil
					break
				}
				select {
				case <-ctx.Done():
				case errc <- err:
				}
				return
			case ctx, ok := <-commands:
				if !ok {
					return
				}
				h.handle(ctx, fn)
			}
		}
	}()

	return <-errc
}

func (h *Handler) handle(ctx command.Context, fn func(context.Context, command.Command) error) {
	handleError := fn(ctx, ctx.Command())

	if handleError != nil {
		h.errs.Publish(
			ctx,
			fmt.Errorf("handle %q command: %w", ctx.Command().Name(), handleError),
		)
	}

	if err := ctx.MarkDone(ctx, done.WithError(handleError)); err != nil {
		h.errs.Publish(ctx, fmt.Errorf("mark as done: %w", err))
	}
}

// Errors returns a channel of asynchronous errors.
func (h *Handler) Errors(ctx context.Context) <-chan error {
	out := make(chan error, 1)
	errs := h.errs.Subscribe(ctx)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for err := range errs {
			out <- err
		}
	}()

	if bus, ok := h.bus.(interface {
		Errors(context.Context) <-chan error
	}); ok {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs := bus.Errors(ctx)
			for err := range errs {
				out <- err
			}
		}()
	}

	go func() {
		wg.Wait()
		close(out)
	}()

	return out
}
