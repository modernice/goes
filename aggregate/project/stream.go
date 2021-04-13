package project

import (
	"context"

	"github.com/modernice/goes/internal/xerror"
)

// Walk receives from the given Context channel until it is closed, ctx is
// closed or any of the provided error channels receives an error. For every
// Context c that is received from the Context channel, walkFn(c) is called.
// Should ctx be canceled before the Context channel is closed, ctx.Err() is
// returned. Should an error be received from one of the optional error
// channels, that error is returned. Otherwise Walk returns nil.
//
// Example:
//
//	var s project.Schedule // create a Schedule
//	var proj project.Projector // create a Projector
//	jobs, errs, err := project.Subscribe(context.TODO(), s, proj)
//	// handle err
//	err := stream.Walk(context.TODO(), func(c project.Context) {
//		var p project.Projection // create a Projection
//		err := c.Project(c, p)
//		// handle err
//	}, jobs, errs)
//	// handle err
func Walk(
	ctx context.Context,
	walkFn func(Context),
	jobs <-chan Context,
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
		case ctx, ok := <-jobs:
			if !ok {
				return nil
			}
			walkFn(ctx)
		}
	}
}

// Drain drains the given Context channel and returns its Contexts.
//
// Drain accepts optional error channels which will cause Drain to fail on any
// error. When Drain encounters an error from any of the error channels, the
// already drained Contexts and that error are returned. Similarly, when ctx is
// canceled, the drained Contexts and ctx.Err() are returned.
//
// Drain returns when the provided Context channel is closed or it encounters an
// error from an error channel and does not wait for the error channels to be
// closed.
func Drain(ctx context.Context, jobs <-chan Context, errs ...<-chan error) ([]Context, error) {
	out := make([]Context, 0, len(jobs))
	err := Walk(ctx, func(c Context) { out = append(out, c) }, jobs, errs...)
	return out, err
}

// ForEvery iterates over the provided Context and error channels and calls ctxFn
// for every received Context and errFn for every received error. ForEvery returns
// when the History and all error channels are closed.
func ForEvery(
	ctxFn func(c Context),
	errFn func(error),
	jobs <-chan Context,
	errs ...<-chan error,
) {
	errChan, stop := xerror.FanIn(errs...)
	defer stop()

	for {
		if errChan == nil && jobs == nil {
			return
		}
		select {
		case err, ok := <-errChan:
			if !ok {
				errChan = nil
				break
			}
			errFn(err)
		case ctx, ok := <-jobs:
			if !ok {
				ctx = nil
				break
			}
			ctxFn(ctx)
		}
	}
}
