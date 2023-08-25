package streams

import (
	"context"
	"errors"
	"sync"
)

// New returns a channel that is filled with the given values. The channel is
// closed after all elements have been pushed into the channel.
func New[T any](in []T) <-chan T {
	out := make(chan T)
	go func() {
		defer close(out)
		for _, v := range in {
			out <- v
		}
	}()
	return out
}

// NewConcurrent creates a channel of the given type and returns the channel and
// a `push` function. The `push` function tries to push a value into the channel
// and accepts a Context to cancel the push operation. The returned `close`
// function closes the channel when called. The `close` function is thread-safe
// and may be called multiple times. If values are provided to NewConcurrent,
// the buffer of the channel is set to the number of values and the values are
// pushed into the channel before returning.
//
//	 str, push, close := NewConcurrent(1, 2, 3)
//	 push(context.TODO(), 4, 5, 6)
//		vals, err := All(str)
//		// handle err
//		// vals == []int{1, 2, 3, 4, 5, 6}
//
// Use the Concurrent function to create a `push` function for an existing channel.
func NewConcurrent[T any](vals ...T) (_ <-chan T, _push func(context.Context, ...T) error, _close func()) {
	var mux sync.Mutex
	var closed bool
	out := make(chan T, len(vals))
	push := Concurrent(out)
	push(context.Background(), vals...)
	return out, push, func() {
		mux.Lock()
		defer mux.Unlock()
		if closed {
			return
		}
		close(out)
		closed = true
	}
}

// Concurrent returns a `push` function for the provided channel.
// The `push` function tries to push values into the channel and accepts
// a Context that can cancel the push operation.
func Concurrent[T any](c chan T) func(context.Context, ...T) error {
	return func(ctx context.Context, vals ...T) error {
		for _, v := range vals {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case c <- v:
			}
		}
		return nil
	}
}

// NewConcurrentContext does the same as NewConcurrent, but uses the provided
// Context for every push call.
func NewConcurrentContext[T any](ctx context.Context, vals ...T) (_ <-chan T, _push func(...T) error, _close func()) {
	out, push, cls := NewConcurrent(vals...)
	return out, func(vals ...T) error {
		return push(ctx, vals...)
	}, cls
}

// ConcurrentContext does the same as Concurrent, but uses the provided Context for every push call.
func ConcurrentContext[T any](ctx context.Context, c chan T) func(...T) error {
	push := Concurrent(c)
	return func(vals ...T) error {
		return push(ctx, vals...)
	}
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

// All drains the given channel and returns its elements.
// All is an alias for Drain(context.Background(), in, errs...).
func All[T any](in <-chan T, errs ...<-chan error) ([]T, error) {
	return Drain(context.Background(), in, errs...)
}

var errTakeDone = errors.New("take done")

// Take receives elements from the input channel until it has received n
// elements or the input channel is closed. It returns a slice containing the
// received elements. If any error occurs during the process, it is returned as
// the second return value.
func Take[T any](ctx context.Context, n int, in <-chan T, errs ...<-chan error) ([]T, error) {
	out := make([]T, 0, n)
	if err := Walk(ctx, func(v T) error {
		out = append(out, v)
		if len(out) >= n {
			return errTakeDone
		}
		return nil
	}, in, errs...); err != nil && !errors.Is(err, errTakeDone) {
		return out, err
	}
	return out, nil
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

// Before returns a new channel that is filled with the elements from the input
// channel. Before sending an element into the returned channel, fn(el) is
// called. The values returned by fn are first sent into the returned channel,
// then the element from the input channel.
//
// If the input channel or fn is nil, the input channel is returned directly.
func Before[Value any](in <-chan Value, fn func(Value) []Value) <-chan Value {
	return BeforeContext(context.Background(), in, fn)
}

// BeforeContext returns a new channel that is filled with the elements from the
// input channel. The returned channel is closed when the input channel is
// closed, or when ctx is canceled. Before sending an element into the returned
// channel, fn(el) is called. The values returned by fn are first sent into the
// returned channel, then the element from the input channel.
//
// If the input channel or fn is nil, the input channel is returned directly.
func BeforeContext[Value any](ctx context.Context, in <-chan Value, fn func(Value) []Value) <-chan Value {
	if in == nil || fn == nil {
		return in
	}

	out := make(chan Value)
	go func() {
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				return
			case el, ok := <-in:
				if !ok {
					return
				}
				for _, v := range fn(el) {
					out <- v
				}
				out <- el
			}
		}
	}()

	return out
}
