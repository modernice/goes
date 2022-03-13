package testutil

import (
	"context"
	"errors"
)

// PanicOn panics on any error that is not nil and does not satisfy
// errors.Is(err, context.Canceled).
func PanicOn(errs <-chan error) {
	for err := range errs {
		if err != nil && !errors.Is(err, context.Canceled) {
			panic(err)
		}
	}
}
