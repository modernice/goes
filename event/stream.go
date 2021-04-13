package event

import (
	"context"

	"github.com/modernice/goes/internal/xerror"
)

// Stream returns an Event channel that is filled and closed with the provided
// Events in a separate goroutine.
func Stream(events ...Event) <-chan Event {
	out := make(chan Event)
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
func Drain(ctx context.Context, events <-chan Event, errs ...<-chan error) ([]Event, error) {
	out := make([]Event, 0, len(events))
	err := Walk(ctx, func(e Event) { out = append(out, e) }, events, errs...)
	return out, err
}

// Walk receives from the given Event channel until it is closed, ctx is closed
// or any of the provided error channels receives an error. For every Event e
// that is received from the Event channel, walkFn(e) is called. Should ctx be
// canceled before the Event channel is closed, ctx.Err() is returned. Should
// an error be received from one of the optional error channels, that error is
// returned. Otherwise Walk returns nil.
//
// Example:
//
//	var bus Bus
//	events, errs, err := bus.Subscribe(context.TODO(), "foo", "bar", "baz")
//	// handle err
//	err := stream.Walk(context.TODO(), func(e Event) {
//		log.Println(fmt.Sprintf("Received %q Event: %v", e.Name(), e))
//	}, events, errs)
//	// handle err
func Walk(
	ctx context.Context,
	walkFn func(Event),
	events <-chan Event,
	errs ...<-chan error,
) error {
	errChan, stop := xerror.FanIn(errs...)
	defer stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err, ok := <-errChan:
			if ok {
				return err
			}
			errChan = nil
		case evt, ok := <-events:
			if !ok {
				return nil
			}
			walkFn(evt)
		}
	}
}

// ForEvery iterates over the provided Event and error channels and calls evtFn
// for every received Event and errFn for every received error. ForEvery returns
// when the Event and all error channels are closed.
func ForEvery(
	evtFn func(evt Event),
	errFn func(error),
	events <-chan Event,
	errs ...<-chan error,
) {
	errChan, stop := xerror.FanIn(errs...)
	defer stop()

	for {
		if errChan == nil && events == nil {
			return
		}
		select {
		case err, ok := <-errChan:
			if !ok {
				errChan = nil
				break
			}
			errFn(err)
		case evt, ok := <-events:
			if !ok {
				events = nil
				break
			}
			evtFn(evt)
		}
	}
}
