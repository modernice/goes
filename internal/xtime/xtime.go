package xtime

import (
	"time"
)

var supportsNanoseconds = false

func init() {
	// Check whether the machine provides nanosecond precision on its own.
	// We check a maximum of 3 times to be sure that we don't do the check
	// precisely on a microsecond where we could get a false negative.
	for i := 0; i < 3; i++ {
		now := time.Now()
		rounded := now.Truncate(time.Microsecond)
		diff := now.Sub(rounded)
		supportsNanoseconds = diff != 0
		if supportsNanoseconds {
			return
		}
	}
}

// SupportsNanoseconds returns whether the machine supports nanosecond precision.
func SupportsNanoseconds() bool {
	return supportsNanoseconds
}

// Now returns the current Time with nanosecond precision. On machines that
// natively provide nanosecond precision Now just returns time.Now(). Otherwise
// Now takes a bit slower path using the monotonic time to calculate the
// nanoseconds manually.
//
// TODO: Remove as soon as go natively supports nanosecond precision on all machines.
func Now() time.Time {
	if supportsNanoseconds {
		return time.Now()
	}

	now := time.Now()
	now2 := time.Now()
	diff := now2.Sub(now)

	return now.Add(diff + time.Microsecond)
}
