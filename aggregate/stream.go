package aggregate

import (
	"context"

	"github.com/modernice/goes/helper/streams"
)

// Deprecated: Use streams.Drain instead.
func Drain(ctx context.Context, str <-chan History, errs ...<-chan error) ([]History, error) {
	return streams.Drain(ctx, str, errs...)
}

func Walk(
	ctx context.Context,
	walkFn func(History) error,
	str <-chan History,
	errs ...<-chan error,
) error {
	return streams.Walk(ctx, walkFn, str, errs...)
}

// Deprecated: Use streams.ForEach instead.
func ForEach(
	ctx context.Context,
	applyFn func(h History),
	errFn func(error),
	histories <-chan History,
	errs ...<-chan error,
) {
	streams.ForEach(ctx, applyFn, errFn, histories, errs...)
}

// Deprecated: Use DrainRefs instead.
func DrainRefs(ctx context.Context, str <-chan Ref, errs ...<-chan error) ([]Ref, error) {
	return streams.Drain(ctx, str, errs...)
}

// Deprecated: Use WalkRefs instead.
var WalkTuples = WalkRefs

// Deprecated: Use streams.Walk instead.
func WalkRefs(
	ctx context.Context,
	walkFn func(Ref) error,
	str <-chan Ref,
	errs ...<-chan error,
) error {
	return streams.Walk(ctx, walkFn, str, errs...)
}

// ForEveryTuple is an alias for ForEachTuple.
//
// Deprecated: Use ForEachRef instead.
var ForEveryTuple = ForEachRef

// Deprecated: Use streams.ForEach instead.
func ForEachRef(
	ctx context.Context,
	applyFn func(Ref),
	errFn func(error),
	refs <-chan Ref,
	errs ...<-chan error,
) {
	streams.ForEach(ctx, applyFn, errFn, refs, errs...)
}
