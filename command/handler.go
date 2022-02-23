package command

import (
	"context"
	"fmt"
	"time"

	"github.com/modernice/goes/command/finish"
	"github.com/modernice/goes/internal/xtime"
)

// Handler can be used to subscribe to and handle commands.
type Handler[P any] struct {
	bus Bus
}

// NewHandler returns a handler for commands that uses the provided Bus to
// subscribe to commands.
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

// Handle registers the function handler as a handler for the given Command name.
// Handle subscribes to the underlying Bus for Commands with that name. When
// Handler is selected as the handler for a dispatched Command, handler is
// called with a Command Context which contains the Command and its Payload.
//
// If Handle fails to subscribe to the Command, a nil channel and the error from
// the Bus is returned. Otherwise a channel of asynchronous Command errors and a
// nil error are returned.
//
// When handler returns a non-nil error, that error is pushed into the returned
// error channel. Asynchronous errors from the underlying Command Bus are pushed
// into the error channel as well. Callers must receive from the returned error
// channel to prevent the handler from blocking indefinitely.
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

// MustHandle does the same as Handle, but panics if the event subscription fails.
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
