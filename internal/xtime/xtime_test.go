package xtime_test

import (
	"testing"
	"time"

	"github.com/modernice/goes/internal/xtime"
)

func TestNow(t *testing.T) {
	timeout := time.NewTimer(3 * time.Second)
	defer timeout.Stop()

	ticker := time.NewTicker(1234 * time.Nanosecond)
	defer ticker.Stop()

L:
	for {
		select {
		case <-timeout.C:
			t.Fatal("timed out")
		case <-ticker.C:
			now := xtime.Now()
			rounded := now.Truncate(time.Microsecond)
			nanoseconds := now.Sub(rounded)

			if nanoseconds != 0 {
				break L
			}
		}
	}
}
