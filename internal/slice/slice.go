package slice

// Map maps the input slice using the provided mapper function.
func Map[In, Out any](in []In, fn func(In) Out) []Out {
	out := make([]Out, len(in))
	for i, v := range in {
		out[i] = fn(v)
	}
	return out
}

// MapErr maps the input slice using the provided mapper function that returns 
// an Out and an error. It returns a new slice of type Out and an error if any 
// occurred during execution. [In] represents the type of input slice and [Out] 
// represents the type of output slice.
func MapErr[In, Out any](in []In, fn func(In) (Out, error)) ([]Out, error) {
	out := make([]Out, len(in))
	var err error
	for i, v := range in {
		out[i], err = fn(v)
		if err != nil {
			return out, err
		}
	}
	return out, nil
}

// IgnoreErr[S []T, T any](s S, err error) S
// 
// IgnoreErr returns the input slice without modifying it. It is used to ignore 
// errors that may be returned from a function call. The function takes in a 
// slice of any type and an error value, and returns the same slice as input.
func IgnoreErr[S []T, T any](s S, err error) S {
	return s
}

// Filter filters the input slice using the provided filter function.
func Filter[T any](in []T, fn func(T) bool) []T {
	out := make([]T, 0, len(in))
	for _, v := range in {
		if fn(v) {
			out = append(out, v)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// Unique returns a new slice containing the unique elements of the input slice.
func Unique[In ~[]T, T comparable](in In) []T {
	out := make([]T, 0, len(in))
	m := make(map[T]struct{})
	for _, v := range in {
		if _, ok := m[v]; !ok {
			out = append(out, v)
			m[v] = struct{}{}
		}
	}
	return out
}
