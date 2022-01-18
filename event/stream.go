package event

import (
	"context"

	"github.com/modernice/goes/helper/streams"
)

// Stream returns an Event channel that is filled and closed with the provided
// Events in a separate goroutine.
func Stream[D any](events ...EventOf[D]) <-chan EventOf[D] {
	out := make(chan EventOf[D])
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
func Drain[D any](ctx context.Context, events <-chan EventOf[D], errs ...<-chan error) ([]EventOf[D], error) {
	out := make([]EventOf[D], 0, len(events))
	err := Walk(ctx, func(e EventOf[D]) error { out = append(out, e); return nil }, events, errs...)
	return out, err
}

// Walk receives from the given Event channel until it and and all provided
// error channels are closed, ctx is closed or any of the provided error
// channels receives an error. For every Event e that is received from the Event
// channel, walkFn(e) is called. Should ctx be canceled before the channels are
// closed, ctx.Err() is returned. Should an error be received from one of the
// error channels, that error is returned. Otherwise Walk returns nil.
//
// Example:
//
//	var bus Bus
//	events, errs, err := bus.Subscribe(context.TODO(), "foo", "bar", "baz")
//	// handle err
//	err := event.Walk(context.TODO(), func(e Event) {
//		log.Println(fmt.Sprintf("Received %q Event: %v", e.Name(), e))
//	}, events, errs)
//	// handle err
func Walk[D any](
	ctx context.Context,
	walkFn func(EventOf[D]) error,
	events <-chan EventOf[D],
	errs ...<-chan error,
) error {
	errChan, stop := streams.FanIn(errs...)
	defer stop()

	for {
		if events == nil && errChan == nil {
			return nil
		}

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
				events = nil
				break
			}
			if err := walkFn(evt); err != nil {
				return err
			}
		}
	}
}

// ForEvery is an alias for ForEach.
//
// Deprecated: Use ForEach instead.
var ForEvery = ForEach[any]

// ForEach iterates over the provided Event and error channels and for every
// Event evt calls evtFn(evt) and for every error e calls errFn(e) until all
// channels are closed or ctx is canceled.
func ForEach[D any](
	ctx context.Context,
	evtFn func(evt EventOf[D]),
	errFn func(error),
	events <-chan EventOf[D],
	errs ...<-chan error,
) {
	errChan, stop := streams.FanIn(errs...)
	defer stop()

	for {
		if errChan == nil && events == nil {
			return
		}

		select {
		case <-ctx.Done():
			return
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

// Filter accepts a channel of Events and returns a filtered channel of Events
// Only Events that test against all provided Queries are pushed into the
// returned channel. The returned channel is closed when the provided channel is
// closed.
func Filter[D any](events <-chan EventOf[D], queries ...Query) <-chan EventOf[D] {
	out := make(chan EventOf[D])
	go func() {
		defer close(out)
	L:
		for evt := range events {
			for _, q := range queries {
				if !Test(q, evt) {
					continue L
				}
			}
			out <- evt
		}
	}()
	return out
}

// Await returns the first Event OR error that is received from events or errs.
// If ctx is canceled before an Event or error is received, ctx.Err() is
// returned.
func Await[D any](ctx context.Context, events <-chan EventOf[D], errs <-chan error) (EventOf[D], error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-errs:
		return nil, err
	case evt := <-events:
		return evt, nil
	}
}
