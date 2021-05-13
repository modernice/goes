package time_test

import (
	"testing"
	stdtime "time"

	"github.com/modernice/goes/event/query/time"
	"github.com/modernice/goes/internal/xtime"
)

func TestExact(t *testing.T) {
	ts := []stdtime.Time{
		xtime.Now(),
		xtime.Now().Add(3 * stdtime.Hour),
		xtime.Now().AddDate(1, 1, 1),
	}
	c := time.Filter(time.Exact(ts...))

	tests := map[stdtime.Time]bool{
		xtime.Now().Add(-stdtime.Second):                      false,
		xtime.Now().Add(3*stdtime.Hour + stdtime.Second):      false,
		xtime.Now().AddDate(1, 1, 1).Add(stdtime.Millisecond): false,
	}

	for _, give := range ts {
		tests[give] = true
	}

	runTests(t, c, tests)
}

func TestInRange(t *testing.T) {
	now := xtime.Now()
	rs := []time.Range{
		{
			now,
			now.Add(3 * stdtime.Hour),
		},
		{
			now.Add(3 * stdtime.Hour),
			now.Add(3*stdtime.Hour + 5*stdtime.Minute),
		},
		{
			now.AddDate(1, 1, 1),
			now.AddDate(1, 1, 3),
		},
	}

	c := time.Filter(time.InRange(rs...))

	tests := map[stdtime.Time]bool{
		now:                                      true,
		now.Add(-stdtime.Second):                 false,
		now.Add(3*stdtime.Hour + stdtime.Minute): true,
		now.AddDate(1, 1, 1):                     true,
		now.AddDate(1, 1, 3):                     true,
		now.AddDate(1, 1, 3).Add(stdtime.Second): false,
	}

	runTests(t, c, tests)
}

func TestBefore(t *testing.T) {
	now := xtime.Now()
	c := time.Filter(time.Before(now))

	tests := map[stdtime.Time]bool{
		now:                        false,
		now.Add(-stdtime.Second):   true,
		now.Add(-2 * stdtime.Hour): true,
	}

	runTests(t, c, tests)
}

func TestAfter(t *testing.T) {
	now := xtime.Now()
	c := time.Filter(time.After(now))

	tests := map[stdtime.Time]bool{
		now:                       false,
		now.Add(stdtime.Second):   true,
		now.Add(2 * stdtime.Hour): true,
	}

	runTests(t, c, tests)
}

func TestMin(t *testing.T) {
	now := xtime.Now()
	c := time.Filter(time.Min(now))

	tests := map[stdtime.Time]bool{
		now.Add(-stdtime.Second):  false,
		now:                       true,
		now.Add(stdtime.Second):   true,
		now.Add(2 * stdtime.Hour): true,
	}

	runTests(t, c, tests)
}

func TestMax(t *testing.T) {
	now := xtime.Now()
	c := time.Filter(time.Max(now))

	tests := map[stdtime.Time]bool{
		now.Add(stdtime.Second):    false,
		now:                        true,
		now.Add(-stdtime.Second):   true,
		now.Add(-2 * stdtime.Hour): true,
	}

	runTests(t, c, tests)
}

func runTests(t *testing.T, c time.Constraints, tests map[stdtime.Time]bool) {
	for give, want := range tests {
		if got := time.Includes(c, give); got != want {
			t.Errorf("time.Includes(c, %v) to return %t; got %t", give, want, got)
		}
	}
}
