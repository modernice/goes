package repository

import (
	"context"
	"fmt"
	"math/rand"
	"time"
)

// Retryer is an aggregate that can retry a failed Repository.Use() operation.
// If the RetryUse() method of the aggregate returns a non-nil RetryTrigger rt,
// failed Repository.Use() calls will be retried until rt.next() returns a
// non-nil error. The returned IsRetryable function is used to determine if the
// error that made Repository.Use() fail is retryable.
//
// The following example retries calls that failed due to consistency errors.
// It is retried every second, up to 3 times before giving up. If the call does
// not succeed after 3 times, the error of the last attempt is returned to the
// caller.
//
//	type Foo struct { *aggregate.Base }
//	func (f *Foo) RetryUse() (rt repository.RetryTrigger, isRetryable repository.IsRetryable) {
//	  return repository.RetryEvery(time.Second, 3), aggregate.IsConsistencyError
//	}
type Retryer interface {
	RetryUse() (RetryTrigger, IsRetryable)
}

// A RetryTrigger triggers a retry of Repository.Use().
type RetryTrigger interface{ next(context.Context) error }

// RetryTriggerFunc allows a function to be used as a RetryTrigger.
type RetryTriggerFunc func(context.Context) error

func (fn RetryTriggerFunc) next(ctx context.Context) error { return fn(ctx) }

// IsRetryable is a function that determines if an error within a
// Reposistory.Use() call is retryable.
type IsRetryable func(error) bool

// A ChangeDiscarder discards changes to the aggregate. The DiscardChanges()
// method is called each time a Repository.Use() call is retried for the
// aggregate.
type ChangeDiscarder interface {
	DiscardChanges()
}

// RetryEvery returns a RetryTrigger that retries every interval up to maxTries.
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

// RetryApprox returns a RetryTrigger that retries approximately every interval
// up to maxTries. The provided deviation is used to randomize the interval. If
// the interval is 1s and deviation is 100ms, then the retry is triggered after
// somewhere between 900ms to 1100ms.
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
