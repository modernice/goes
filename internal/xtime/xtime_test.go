package xtime_test

import (
	"testing"
	"time"

	"github.com/modernice/goes/internal/xtime"
)

func TestNow(t *testing.T) {
	now := xtime.Now()
	rounded := now.Truncate(time.Microsecond)
	nanoseconds := now.Sub(rounded)
	if nanoseconds == 0 {
		t.Fatalf("Now should return Time with nanosecond precision; got %v (%d)", now, now.UnixNano())
	}
}
