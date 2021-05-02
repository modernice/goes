package command

import (
	"context"
	"fmt"
	"time"

	"github.com/modernice/goes/command/finish"
)

// Handler can be used to subscribe to and handle Commands.
type Handler struct {
	bus Bus
}

// NewHandler returns a Handler for Commands that uses the provided Bus to
// subscribe to Commands.
func NewHandler(bus Bus) *Handler {
	return &Handler{bus}
}

// Handle is a shortcut for
//	NewHandler(bus).Handle(ctx, name, handler)
func Handle(ctx context.Context, bus Bus, name string, handler func(Context) error) (<-chan error, error) {
	return NewHandler(bus).Handle(ctx, name, handler)
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
func (h *Handler) Handle(ctx context.Context, name string, handler func(Context) error) (<-chan error, error) {
	str, errs, err := h.bus.Subscribe(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("subscribe to %v Command: %w", name, err)
	}

	out := make(chan error)
	go h.handle(ctx, handler, str, errs, out)

	return out, nil
}

func (h *Handler) handle(
	ctx context.Context,
	handler func(Context) error,
	str <-chan Context,
	errs <-chan error,
	out chan<- error,
) {
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
			case out <- fmt.Errorf("Command subscription: %w", err):
			}
		case ctx, ok := <-str:
			if !ok {
				str = nil
				break
			}

			start := time.Now()
			err := handler(ctx)
			runtime := time.Since(start)

			cmd := ctx.Command()

			if err != nil {
				select {
				case <-ctx.Done():
					return
				case out <- fmt.Errorf("handle %q Command: %w", cmd.Name(), err):
				}
			}

			if err := ctx.Finish(ctx, finish.WithError(err), finish.WithRuntime(runtime)); err != nil {
				select {
				case <-ctx.Done():
					return
				case out <- fmt.Errorf("finish %q Command: %w", cmd.Name(), err):
				}
			}
		}
	}
}
