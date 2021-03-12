package stream

import (
	"context"

	"github.com/modernice/goes/aggregate/snapshot"
	"github.com/modernice/goes/internal/xerror"
)

// Drain drains the given Snapshot channel and returns its Snapshots.
//
// Drain accepts optional error channels which will cause Drain to fail on any
// error. When Drain encounters an error from any of the error channels, the
// already drained Snapshots and that error are returned. Similarly, when ctx is
// canceled, the drained Snapshots and ctx.Err() are returned.
//
// Drain returns when the provided Snapshot channel is closed or it encounters an
// error from an error channel and does not wait for the error channels to be
// closed.
func Drain(
	ctx context.Context,
	snaps <-chan snapshot.Snapshot,
	errs ...<-chan error,
) ([]snapshot.Snapshot, error) {
	out := make([]snapshot.Snapshot, 0, len(snaps))
	err := Walk(
		ctx,
		func(s snapshot.Snapshot) { out = append(out, s) },
		snaps,
		errs...,
	)
	return out, err
}

// Walk retrieves from the given Snapshot channel until it is closed, ctx is
// closed or any of the provided error channels receives an error. For every
// Snapshot s that is received from the Snapshot channel, walkFn(s) is called.
// Should ctx be canceled before the Snapshot channel is closed, ctx.Err() is
// returned. Should an error be received from one of the optional error
// channels, that error is returned. Otherwise Walk returns nil.
//
// Example:
//
//	var store snapshot.Store
//	snaps, errs, err := store.Query(context.TODO(), query.New())
//	// handle err
//	err := stream.Walk(context.TODO(), func(s snapshot.Snapshot) {
//		log.Println(fmt.Sprintf("Received Snapshot: %v", s))
//	}, snaps, errs)
//	// handle err
func Walk(
	ctx context.Context,
	walkFn func(snapshot.Snapshot),
	snaps <-chan snapshot.Snapshot,
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
		case snap, ok := <-snaps:
			if !ok {
				return nil
			}
			walkFn(snap)
		}
	}
}
