package eventbus

import (
	"context"
	"fmt"

	"github.com/modernice/goes"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/internal/concurrent"
)

// Await is a shortcut for NewAwaiter(bus).Once(ctx, names...). See Awaiter.Once
// for documentation.
func Await[D any, ID goes.ID](ctx context.Context, bus event.Bus[ID], names ...string) (<-chan event.Of[D, ID], <-chan error, error) {
	return NewAwaiter[D, ID](bus).Once(ctx, names...)
}

// Awaiter can be used to await events in more complex scenarios.
type Awaiter[D any, ID goes.ID] struct {
	bus event.Bus[ID]
}

// NewAwaiter returns an Awaiter for the given Bus.
func NewAwaiter[D any, ID goes.ID](bus event.Bus[ID]) Awaiter[D, ID] {
	return Awaiter[D, ID]{bus}
}

// Once subscribes to the given events over the event bus and returns a channel
// for the event and an error channel. The returned Event channels will never
// receive more than one element (either a single Event or a single error). When
// an Event or an error is received, both channels are immediately closed.
//
// If len(names) == 0, Once returns nil channels.
func (a Awaiter[D, ID]) Once(ctx context.Context, names ...string) (<-chan event.Of[D, ID], <-chan error, error) {
	if len(names) == 0 {
		return nil, nil, nil
	}

	ctx, cancel := context.WithCancel(ctx)

	events, errs, err := a.bus.Subscribe(ctx, names...)
	if err != nil {
		cancel()
		return nil, nil, fmt.Errorf("subscribe to %q events: %w", names, err)
	}

	out := make(chan event.Of[D, ID])
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
			casted, ok := event.TryCast[D, any, ID](evt)
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
