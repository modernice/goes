package snapshot

import (
	"context"

	"github.com/modernice/goes/helper/streams"
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
	snaps <-chan Snapshot,
	errs ...<-chan error,
) ([]Snapshot, error) {
	out := make([]Snapshot, 0, len(snaps))
	err := Walk(ctx, func(s Snapshot) error { out = append(out, s); return nil }, snaps, errs...)
	return out, err
}

// Walk retrieves from the given Snapshot channel until it and the error
// channels are closed, ctx is canceled or any of the provided error channels
// receives an error. For every Snapshot s that is received from the Snapshot
// channel, walkFn(s) is called. Should ctx be canceled before the Snapshot
// channel is closed, ctx.Err() is returned. Should an error be received from
// one of the error channels, that error is returned. Otherwise Walk returns nil.
//
// Example:
//
//	var store snapshot.Store
//	snaps, errs, err := store.Query(context.TODO(), query.New())
//	// handle err
//	err := snapshot.Walk(context.TODO(), func(s snapshot.Snapshot) {
//		log.Println(fmt.Sprintf("Received Snapshot: %v", s))
//		return nil
//	}, snaps, errs)
//	// handle err
func Walk(
	ctx context.Context,
	walkFn func(Snapshot) error,
	snaps <-chan Snapshot,
	errs ...<-chan error,
) error {
	errChan, stop := streams.FanIn(errs...)
	defer stop()

	for {
		if snaps == nil && errChan == nil {
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
		case snap, ok := <-snaps:
			if !ok {
				snaps = nil
				break
			}
			if err := walkFn(snap); err != nil {
				return err
			}
		}
	}
}

// ForEvery is an alias for ForEach.
//
// Deprecated: Use ForEach instead.
var ForEvery = ForEach

// ForEach iterates over the given Snapshot and error channels and for every
// Snapshot s calls snapFn(s) and for every error e calls errorFn(e) until all
// channels are closed or ctx is canceled.
func ForEach(
	ctx context.Context,
	snapFn func(Snapshot),
	errFn func(error),
	str <-chan Snapshot,
	errs ...<-chan error,
) {
	errChan, stop := streams.FanIn(errs...)
	defer stop()

	for {
		if str == nil && errChan == nil {
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
		case snap, ok := <-str:
			if !ok {
				str = nil
				break
			}
			snapFn(snap)
		}
	}
}
