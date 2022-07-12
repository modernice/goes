package errstream

import "fmt"

// Prefix adds a message prefix to each error in the input channel.
func Prefix(prefix string, in <-chan error) <-chan error {
	out := make(chan error)
	go func() {
		defer close(out)
		for err := range in {
			out <- fmt.Errorf("%s%w", prefix, err)
		}
	}()
	return out
}

// Module adds a "module" prefix to each error in the input channel.
// The errors returned by the output channel are formatted as:
//   fmt.Errorf("[%s] %w", name, err)
func Module(name string, in <-chan error) <-chan error {
	return Prefix(fmt.Sprintf("[%s] ", name), in)
}
