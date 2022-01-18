package aggregate

import (
	"context"

	"github.com/modernice/goes/helper/fanin"
)

// Drain drains the given History channel and returns its Histories.
//
// Drain accepts optional error channels which will cause Drain to fail on any
// error. When Drain encounters an error from any of the error channels, the
// already drained Histories and that error are returned. Similarly, when ctx is
// canceled, the drained Histories and ctx.Err() are returned.
//
// Drain returns when the provided History channel is closed or it encounters an
// error from an error channel and does not wait for the error channels to be
// closed.
func Drain(ctx context.Context, str <-chan History, errs ...<-chan error) ([]History, error) {
	out := make([]History, 0, len(str))
	err := Walk(ctx, func(h History) error { out = append(out, h); return nil }, str, errs...)
	return out, err
}

// Walk receives from the given History channel until it and all provided error
// channels are closed, ctx is closed or any of the provided error channels receives
// an error. For every History h that is received from the History channel,
// walkFn(h) is called. Should ctx be canceled before the channels are closed,
// ctx.Err() is returned. Should an error be received from one of the error
// channels, that error is returned. Otherwise Walk returns nil.
//
// Example:
//
//	var repo aggregate.Repository
//	str, errs, err := repo.Query(context.TODO(), query.New())
//	// handle err
//	err := aggregate.Walk(context.TODO(), func(h aggregate.History) {
//		log.Println(fmt.Sprintf("Received History: %v", h))
//	}, str, errs)
//	// handle err
func Walk(
	ctx context.Context,
	walkFn func(History) error,
	str <-chan History,
	errs ...<-chan error,
) error {
	errChan, stop := fanin.Errors(errs...)
	defer stop()

	for {
		if str == nil && errChan == nil {
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
		case his, ok := <-str:
			if !ok {
				str = nil
				break
			}
			if err := walkFn(his); err != nil {
				return err
			}
		}
	}
}

// ForEach iterates over the provided History and error channels and for every
// History h calls applyFn(h) and for every error e calls errFn(e) until all
// channels are closed or ctx is canceled.
func ForEach(
	applyFn func(h History),
	errFn func(error),
	histories <-chan History,
	errs ...<-chan error,
) {
	errChan, stop := fanin.Errors(errs...)
	defer stop()

	for {
		if errChan == nil && histories == nil {
			return
		}

		select {
		case err, ok := <-errChan:
			if !ok {
				errChan = nil
				break
			}
			errFn(err)
		case h, ok := <-histories:
			if !ok {
				histories = nil
				break
			}
			applyFn(h)
		}
	}
}

// DrainRefs drains the given Ref channel and returns its Refs.
//
// DrainRefs accepts optional error channels which will cause DrainRefs to
// fail on any error. When DrainRefs encounters an error from any of the error
// channels, the already drained Refs and that error are returned. Similarly,
// when ctx is canceled, the drained Refs and ctx.Err() are returned.
//
// DrainRefs returns when the provided Ref channel is closed or it
// encounters an error from an error channel and does not wait for the error
// channels to be closed.
func DrainRefs(ctx context.Context, str <-chan Ref, errs ...<-chan error) ([]Ref, error) {
	out := make([]Ref, 0, len(str))
	err := WalkRefs(ctx, func(r Ref) error { out = append(out, r); return nil }, str, errs...)
	return out, err
}

// Deprecated: Use RefWalk instead.
var WalkTuples = WalkRefs

// WalkRefs receives from the given Ref channel until it and all provided
// error channels are closed, ctx is closed or any of the provided error
// channels receives an error. For every Ref r that is received from the Ref
// channel, walkFn(h) is called. Should ctx be canceled before the channels are
// closed, ctx.Err() is returned. Should an error be received from one of the
// error channels, that error is returned. Otherwise Walk returns nil.
func WalkRefs(
	ctx context.Context,
	walkFn func(Ref) error,
	str <-chan Ref,
	errs ...<-chan error,
) error {
	errChan, stop := fanin.Errors(errs...)
	defer stop()

	for {
		if str == nil && errChan == nil {
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
		case r, ok := <-str:
			if !ok {
				str = nil
				break
			}
			if err := walkFn(r); err != nil {
				return err
			}
		}
	}
}

// ForEveryTuple is an alias for ForEachTuple.
//
// Deprecated: Use ForEachRef instead.
var ForEveryTuple = ForEachRef

// ForEachRef iterates over the provided Ref and error channels and for
// every Ref r calls applyFn(r) and for every error e calls errFn(e) until
// all channels are closed or ctx is canceled.
func ForEachRef(
	applyFn func(Ref),
	errFn func(error),
	refs <-chan Ref,
	errs ...<-chan error,
) {
	errChan, stop := fanin.Errors(errs...)
	defer stop()

	for {
		if errChan == nil && refs == nil {
			return
		}

		select {
		case err, ok := <-errChan:
			if !ok {
				errChan = nil
				break
			}
			errFn(err)
		case r, ok := <-refs:
			if !ok {
				refs = nil
				break
			}
			applyFn(r)
		}
	}
}
