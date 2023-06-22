package repository

import (
	"context"
	"fmt"
	"math/rand"
	"time"
)

// Retryer is an interface for providing a retry mechanism in operations. It
// consists of a RetryTrigger, which determines the timing of the next attempt,
// and an IsRetryable function, which checks if an error should be retried.
type Retryer interface {
	// RetryUse returns a RetryTrigger and an IsRetryable function for use in
	// retrying operations. The RetryTrigger determines the next attempt's timing,
	// and the IsRetryable function checks if an error is retryable.
	RetryUse() (RetryTrigger, IsRetryable)
}

// RetryTrigger is an interface that defines a method for determining the next
// attempt's timing in a retryable operation. It is used in conjunction with
// [IsRetryable] to implement custom retry strategies within a [Retryer]
// interface.
type RetryTrigger interface{ next(context.Context) error }

// RetryTriggerFunc is a function type that implements the RetryTrigger
// interface, allowing users to define custom logic for determining the timing
// of retry attempts in a Retryer. It takes a context as an input and returns an
// error if any occurs during its execution.
type RetryTriggerFunc func(context.Context) error

func (fn RetryTriggerFunc) next(ctx context.Context) error { return fn(ctx) }

// IsRetryable is a function type that determines if an error should trigger a
// retry. It returns true if the error is considered retryable, and false
// otherwise. It is typically used in conjunction with [RetryTrigger] to
// implement custom retry strategies in a [Retryer] interface.
type IsRetryable func(error) bool

// ChangeDiscarder is an interface that provides a method for discarding changes
// in a repository.
type ChangeDiscarder interface {
	// DiscardChanges discards any changes made to the underlying data within the
	// ChangeDiscarder, effectively reverting it to its original state before the
	// changes were made.
	DiscardChanges()
}

// RetryEvery returns a RetryTrigger that retries an operation at a fixed
// interval for a specified number of maxTries. The operation will be retried
// until the maximum number of tries is reached, or the provided context is
// canceled.
func RetryEvery(interval time.Duration, maxTries int) RetryTrigger {
	tries := 1
	return RetryTriggerFunc(func(ctx context.Context) error {
		if tries >= maxTries {
			return fmt.Errorf("tried %d times", tries)
		}

		timer := time.NewTimer(interval)
		defer timer.Stop()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			tries++
			return nil
		}
	})
}

// RetryApprox creates a RetryTrigger that retries an operation with a
// randomized interval between attempts. The interval is calculated by adding a
// random percentage of deviation to the base interval, and the maximum number
// of attempts is specified by maxTries.
func RetryApprox(interval, deviation time.Duration, maxTries int) RetryTrigger {
	tries := 1
	return RetryTriggerFunc(func(ctx context.Context) error {
		if tries >= maxTries {
			return fmt.Errorf("tried %d times", tries)
		}

		sign := 1
		if rand.Intn(2) == 0 {
			sign = -1
		}

		perc := rand.Intn(101)

		dev := deviation * time.Duration(perc) * time.Duration(sign) / 100
		iv := interval + dev

		timer := time.NewTimer(iv)
		defer timer.Stop()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			tries++
			return nil
		}
	})
}
