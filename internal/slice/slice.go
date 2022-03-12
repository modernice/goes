package slice

// Map maps the input slice using the provided mapper function.
func Map[In, Out any](in []In, fn func(In) Out) []Out {
	out := make([]Out, len(in))
	for i, v := range in {
		out[i] = fn(v)
	}
	return out
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
