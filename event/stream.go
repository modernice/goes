package event

import (
	"context"

	"github.com/modernice/goes/helper/streams"
)

// Deprecated: Use streams.New instead.
func Stream[D any](events ...EventOf[D]) <-chan EventOf[D] {
	return streams.New(events...)
}

// Deprecated: Use streams.Drain instead.
func Drain[D any](ctx context.Context, events <-chan EventOf[D], errs ...<-chan error) ([]EventOf[D], error) {
	return streams.Drain(ctx, events, errs...)
}

func Walk[D any](
	ctx context.Context,
	walkFn func(EventOf[D]) error,
	events <-chan EventOf[D],
	errs ...<-chan error,
) error {
	return streams.Walk(ctx, walkFn, events, errs...)
}

// ForEvery is an alias for ForEach.
//
// Deprecated: Use ForEach instead.
var ForEvery = ForEach[any]

// Deprecated: Use streams.ForEach instead.
func ForEach[D any](
	ctx context.Context,
	evtFn func(evt EventOf[D]),
	errFn func(error),
	events <-chan EventOf[D],
	errs ...<-chan error,
) {
	streams.ForEach(ctx, evtFn, errFn, events, errs...)
}

func Filter[D any](events <-chan EventOf[D], queries ...Query) <-chan EventOf[D] {
	filters := make([]func(EventOf[D]) bool, len(queries))
	for i, q := range queries {
		filters[i] = func(evt EventOf[D]) bool {
			return Test(q, evt)
		}
	}
	return streams.Filter(events)
}

// Deprecated: Use streams.Await instead.
func Await[D any](ctx context.Context, events <-chan EventOf[D], errs <-chan error) (EventOf[D], error) {
	return streams.Await(ctx, events, errs)
}
