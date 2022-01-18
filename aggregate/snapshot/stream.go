package snapshot

import (
	"context"

	"github.com/modernice/goes/helper/streams"
)

// Deprecated: Use streams.Drain instead.
func Drain(
	ctx context.Context,
	snaps <-chan Snapshot,
	errs ...<-chan error,
) ([]Snapshot, error) {
	return streams.Drain(ctx, snaps, errs...)
}

// Deprecated: Use streams.Walk instead.
func Walk(
	ctx context.Context,
	walkFn func(Snapshot) error,
	snaps <-chan Snapshot,
	errs ...<-chan error,
) error {
	return streams.Walk(ctx, walkFn, snaps, errs...)
}

// ForEvery is an alias for ForEach.
//
// Deprecated: Use ForEach instead.
var ForEvery = ForEach

// Deprecated: Use streams.ForEach instead.
func ForEach(
	ctx context.Context,
	snapFn func(Snapshot),
	errFn func(error),
	str <-chan Snapshot,
	errs ...<-chan error,
) {
	streams.ForEach(ctx, snapFn, errFn, str, errs...)
}
