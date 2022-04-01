package streams

import "context"

// New returns a channel of the provided variadic type and fills it with the
// provided elements. The channel is closed after all elements have been pushed
// into the channel.
func New[T any](in ...T) <-chan T {
	out := make(chan T)
	go func() {
		defer close(out)
		for _, v := range in {
			out <- v
		}
	}()
	return out
}

// Drain drains the given channel and returns its elements.
//
// Drain accepts optional error channels which will cause Drain to fail on any
// error. When Drain encounters an error, the already drained elements and the
// error are returned. Similarly, when ctx is canceled, the drained elements and
// ctx.Err() are returned.
//
// Drain returns when the input channel is closed or if it encounters an error
// and does not wait for the error channels to be closed.
func Drain[T any](ctx context.Context, in <-chan T, errs ...<-chan error) ([]T, error) {
	out := make([]T, 0, len(in))
	err := Walk(ctx, func(v T) error { out = append(out, v); return nil }, in, errs...)
	return out, err
}

// Walk receives from the given channel until it and and all provided error
// channels are closed, ctx is closed or any of the provided error channels
// receives an error. For every element e that is received from the input
// channel, walkFn(e) is called. Should ctx be canceled before the channels are
// closed, ctx.Err() is returned. Should an error be received from one of the
// error channels, that error is returned. Otherwise Walk returns nil.
//
// Example:
//
//	var bus event.Bus
//	in, errs, err := bus.Subscribe(context.TODO(), "foo", "bar", "baz")
//	// handle err
//	err := Walk(context.TODO(), func(e event) {
//		log.Println(fmt.Sprintf("Received %q event: %v", e.Name(), e))
//	}, in, errs)
//	// handle err
func Walk[T any](
	ctx context.Context,
	walkFn func(T) error,
	in <-chan T,
	errs ...<-chan error,
) error {
	errChan, stop := FanIn(errs...)
	defer stop()

	for {
		if in == nil && errChan == nil {
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
		case el, ok := <-in:
			if !ok {
				in = nil
				break
			}
			if err := walkFn(el); err != nil {
				return err
			}
		}
	}
}

// ForEach iterates over the provided channels and for every element e calls
// calls fn(e) and for every error e calls errFn(e) until all channels are
// closed or ctx is canceled.
func ForEach[T any](
	ctx context.Context,
	fn func(el T),
	errFn func(error),
	in <-chan T,
	errs ...<-chan error,
) {
	errChan, stop := FanIn(errs...)
	defer stop()

	for {
		if errChan == nil && in == nil {
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
		case el, ok := <-in:
			if !ok {
				in = nil
				break
			}
			fn(el)
		}
	}
}

// Filter returns a new channel with the same type as the input channel and
// fills it with the elements from the input channel. The provided filter
// functions are called for every element. If any of the filters returns false
// for an element, that element is not pushed into the returned channel.
func Filter[T any](in <-chan T, filters ...func(T) bool) <-chan T {
	out := make(chan T)
	go func() {
		defer close(out)
	L:
		for el := range in {
			for _, f := range filters {
				if !f(el) {
					continue L
				}
			}
			out <- el
		}
	}()
	return out
}

// Await awaits the next element or error (whatever happens first) from the
// provided channels. Await returns either an element OR an error, never both.
// If ctx is canceled before an element is received, ctx.Err() is returned.
func Await[T any](ctx context.Context, in <-chan T, errs <-chan error) (T, error) {
	var zero T
	select {
	case <-ctx.Done():
		return zero, ctx.Err()
	case err := <-errs:
		return zero, err
	case el := <-in:
		return el, nil
	}
}

// Map maps the elements from the provided `in` channel using the provided
// `mapper` and sends the mapped values to the returned channel. The returned
// channel is closed when the input channel is closed or ctx is canceled.
func Map[To, From any](ctx context.Context, in <-chan From, mapper func(From) To) <-chan To {
	out := make(chan To)
	go func() {
		defer close(out)
		for v := range in {
			select {
			case <-ctx.Done():
				return
			case out <- mapper(v):
			}
		}
	}()
	return out
}
