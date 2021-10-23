package eventbus

import (
	"context"
	"fmt"

	"github.com/modernice/goes/event"
	"github.com/modernice/goes/internal/concurrent"
)

// Await is a shortcut for NewAwaiter(bus).Once(ctx, names...). See Awaiter.Once
// for documentation.
func Await(ctx context.Context, bus event.Bus, names ...string) (<-chan event.Event, <-chan error, error) {
	return NewAwaiter(bus).Once(ctx, names...)
}

// Awaiter can be used to await events in more complex scenarios.
type Awaiter struct {
	bus event.Bus
}

// NewAwaiter returns an Awaiter for the given Bus.
func NewAwaiter(bus event.Bus) Awaiter {
	return Awaiter{bus}
}

// Once subscribes to the given events over the event bus and returns a channel
// for the event and an error channel. The returned Event channels will never
// receive more than one element (either a single Event or a single error). When
// an Event or an error is received, both channels are immediately closed.
//
// If len(names) == 0, Once returns nil channels.
func (a Awaiter) Once(ctx context.Context, names ...string) (<-chan event.Event, <-chan error, error) {
	if len(names) == 0 {
		return nil, nil, nil
	}

	ctx, cancel := context.WithCancel(ctx)

	events, errs, err := a.bus.Subscribe(ctx, names...)
	if err != nil {
		cancel()
		return nil, nil, fmt.Errorf("subscribe to %q events: %w", names, err)
	}

	out := make(chan event.Event)
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
		case out <- <-events:
		}
	}()

	return out, outErrs, nil
}
