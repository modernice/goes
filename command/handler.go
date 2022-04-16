package command

import (
	"context"
	"fmt"
	"time"

	"github.com/modernice/goes/command/finish"
	"github.com/modernice/goes/internal/xtime"
)

// Handler wraps a Bus to provide a convenient way to subscribe to and handle commands.
type Handler[P any] struct {
	bus Bus
}

// NewHandler wraps the provided Bus in a *Handler.
func NewHandler[P any](bus Bus) *Handler[P] {
	return &Handler[P]{bus}
}

// Handle is a shortcut for
//	NewHandler(bus).Handle(ctx, name, handler)
func Handle[P any](ctx context.Context, bus Bus, name string, handler func(Ctx[P]) error) (<-chan error, error) {
	return NewHandler[P](bus).Handle(ctx, name, handler)
}

// MustHandle is a shortcut for
//	NewHandler(bus).MustHandle(ctx, name, handler)
func MustHandle[P any](ctx context.Context, bus Bus, name string, handler func(Ctx[P]) error) <-chan error {
	return NewHandler[P](bus).MustHandle(ctx, name, handler)
}

// Handle registers the provided function as a handler for the given command.
// Handle subscribes to the command over the underlying Bus. The command.Context
// returned by the Bus is passed to the provided handler function. Afterwards,
// the `Finish` method of the command.Context is called by *Handler to report
// the execution result of the command.
//
// Handle returns a channel of asynchronous errors. Users are responsible for
// receiving the errors from the channel, to avoid blocking. Errors that are
// sent into the channel are
//
//	- all asynchronous errors from the underlying Bus
// 	- all errors returned by the provided handler function
//	- errors returned by the `Finish` method of command.Context
//
// When ctx is canceled, the returned error channel is closed.
func (h *Handler[P]) Handle(ctx context.Context, name string, handler func(Ctx[P]) error) (<-chan error, error) {
	str, errs, err := h.bus.Subscribe(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("subscribe to %v Command: %w", name, err)
	}

	out := make(chan error)
	go h.handle(ctx, handler, str, errs, out)

	return out, nil
}

// MustHandle does the same as Handle, but panics if the command subscription fails.
func (h *Handler[P]) MustHandle(ctx context.Context, name string, handler func(Ctx[P]) error) <-chan error {
	errs, err := h.Handle(ctx, name, handler)
	if err != nil {
		panic(err)
	}
	return errs
}

func (h *Handler[P]) handle(
	ctx context.Context,
	handler func(Ctx[P]) error,
	str <-chan Context,
	errs <-chan error,
	out chan<- error,
) {
	defer close(out)
	for {
		if str == nil && errs == nil {
			return
		}

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
				return
			case out <- fmt.Errorf("command subscription: %w", err):
			}
		case ctx, ok := <-str:
			if !ok {
				str = nil
				break
			}

			casted, ok := TryCastContext[P](ctx)
			if !ok {
				select {
				case <-ctx.Done():
					return
				case out <- fmt.Errorf("failed to cast context [from=%T, to=%T]", ctx, casted):
				}
			}

			start := xtime.Now()
			err := handler(casted)
			runtime := time.Since(start)

			cmd := ctx

			if err != nil {
				select {
				case <-ctx.Done():
					return
				case out <- fmt.Errorf("handle %q command: %w", cmd.Name(), err):
				}
			}

			if err := ctx.Finish(ctx, finish.WithError(err), finish.WithRuntime(runtime)); err != nil {
				select {
				case <-ctx.Done():
					return
				case out <- fmt.Errorf("finish %q command: %w", cmd.Name(), err):
				}
			}
		}
	}
}
