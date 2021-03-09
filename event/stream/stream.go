package stream

import (
	"context"

	"github.com/modernice/goes/event"
	"github.com/modernice/goes/internal/xerror"
)

// Slice returns an Event channel that is filled and closed with the provided
// Events in a separate goroutine.
func Slice(events ...event.Event) <-chan event.Event {
	out := make(chan event.Event)
	go func() {
		defer close(out)
		for _, evt := range events {
			out <- evt
		}
	}()
	return out
}

// Drain drains the given Event channel and returns its Events.
//
// Drain accepts optional error channels which will cause Drain to fail on any
// error. When Drain encounters an error from any of the error channels, the
// already drained Events and that error are returned. Similarly, when ctx is
// canceled, the drained Events and ctx.Err() are returned.
//
// Drain returns when the provided Event channel is closed or it encounters an
// error from an error channel and does not wait for the error channels to be
// closed.
func Drain(ctx context.Context, events <-chan event.Event, errs ...<-chan error) ([]event.Event, error) {
	errChan, stop := xerror.FanIn(errs...)
	defer stop()

	out := make([]event.Event, 0, len(events))
	for {
		select {
		case <-ctx.Done():
			return out, ctx.Err()
		case err, ok := <-errChan:
			if ok {
				return out, err
			}
			errChan = nil
		case evt, ok := <-events:
			if !ok {
				return out, nil
			}
			out = append(out, evt)
		}
	}
}
