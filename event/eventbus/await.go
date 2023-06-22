package eventbus

import (
	"context"
	"fmt"

	"github.com/modernice/goes/event"
	"github.com/modernice/goes/internal/concurrent"
)

// Await returns a channel that receives the first occurrence of an event with
// one of the specified names from the provided event bus, along with a channel
// for errors and a possible error. The context is used to cancel the operation
// if needed. The returned channels should be read from to prevent goroutine
// leaks.
func Await[D any](ctx context.Context, bus event.Bus, names ...string) (<-chan event.Of[D], <-chan error, error) {
	return NewAwaiter[D](bus).Once(ctx, names...)
}

// Awaiter is a type that provides functionality to wait for specific events
// from an event.Bus, and then delivers them as typed events via channels. It
// supports cancellation through the provided context.Context.
type Awaiter[D any] struct {
	bus event.Bus
}

// NewAwaiter creates and returns a new Awaiter instance that uses the provided
// event.Bus to subscribe and wait for events.
func NewAwaiter[D any](bus event.Bus) Awaiter[D] {
	return Awaiter[D]{bus}
}

// Once listens for the first occurrence of the specified events in the context
// and returns a channel that emits the event, an error channel, and an error.
// If no event names are provided, it returns nil channels and no error. The
// context is used to cancel the operation if necessary.
func (a Awaiter[D]) Once(ctx context.Context, names ...string) (<-chan event.Of[D], <-chan error, error) {
	if len(names) == 0 {
		return nil, nil, nil
	}

	ctx, cancel := context.WithCancel(ctx)

	events, errs, err := a.bus.Subscribe(ctx, names...)
	if err != nil {
		cancel()
		return nil, nil, fmt.Errorf("subscribe to %q events: %w", names, err)
	}

	out := make(chan event.Of[D])
	outErrs, fail := concurrent.Errors(ctx)

	go func() {
		defer close(out)
		defer cancel()

		select {
		case <-ctx.Done():
			fail(ctx.Err())
			return
		case err := <-errs:
			fail(err)
			return
		case evt := <-events:
			casted, ok := event.TryCast[D](evt)
			if !ok {
				var to D
				fail(fmt.Errorf("failed to cast event [from=%T, to=%T]", evt, to))
				return
			}

			select {
			case <-ctx.Done():
				fail(ctx.Err())
				return
			case out <- casted:
			}
		}
	}()

	return out, outErrs, nil
}
