package event

import (
	"context"

	"github.com/modernice/goes/helper/fanin"
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
	err := Walk(ctx, func(e Event) error { out = append(out, e); return nil }, events, errs...)
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
func Walk(
	ctx context.Context,
	walkFn func(Event) error,
	events <-chan Event,
	errs ...<-chan error,
) error {
	errChan, stop := fanin.Errors(errs...)
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
var ForEvery = ForEach

// ForEach iterates over the provided Event and error channels and for every
// Event evt calls evtFn(evt) and for every error e calls errFn(e) until all
// channels are closed or ctx is canceled.
func ForEach(
	ctx context.Context,
	evtFn func(evt Event),
	errFn func(error),
	events <-chan Event,
	errs ...<-chan error,
) {
	errChan, stop := fanin.Errors(errs...)
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
func Filter(events <-chan Event, queries ...Query) <-chan Event {
	out := make(chan Event)
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
func Await(ctx context.Context, events <-chan Event, errs <-chan error) (Event, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-errs:
		return nil, err
	case evt := <-events:
		return evt, nil
	}
}
