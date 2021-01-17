package version_test

import (
	"reflect"
	"testing"

	"github.com/modernice/goes/event/version"
)

func TestExact(t *testing.T) {
	v := []int{1, 3, 6}
	c := version.Filter(version.Exact(v...))

	if !reflect.DeepEqual(c.Exact(), v) {
		t.Fatalf("c.Exact() should return %v; got %v", v, c.Exact())
	}

	tests := map[int]bool{
		1:    true,
		3:    true,
		6:    true,
		0:    false,
		2:    false,
		4:    false,
		5:    false,
		7:    false,
		100:  false,
		-100: false,
	}

	runTests(t, c, tests)
}

func TestInRange(t *testing.T) {
	rs := []version.Range{
		{2, 18},
		{22, 31},
		{28, 40},
	}

	c := version.Filter(version.InRange(rs...))

	if !reflect.DeepEqual(c.Ranges(), rs) {
		t.Fatalf("c.Ranges() should return %v; got %v", rs, c.Ranges())
	}

	tests := map[int]bool{
		0:  false,
		1:  false,
		2:  true,
		10: true,
		17: true,
		18: true,
		19: false,
		21: false,
		22: true,
		28: true,
		30: true,
		31: true,
		39: true,
		40: true,
		41: false,
	}

	runTests(t, c, tests)
}

func TestMin(t *testing.T) {
	min := []int{3, 7, 10}
	c := version.Filter(version.Min(min...))

	if !reflect.DeepEqual(c.Min(), min) {
		t.Fatalf("c.Min() should return %v; got %v", min, c.Min())
	}

	tests := map[int]bool{
		0:   false,
		1:   false,
		2:   false,
		3:   true,
		5:   true,
		7:   true,
		100: true,
	}

	runTests(t, c, tests)
}

func TestMax(t *testing.T) {
	max := []int{30, 70, 100}
	c := version.Filter(version.Max(max...))

	if !reflect.DeepEqual(c.Max(), max) {
		t.Fatalf("c.Max() should return %v; got %v", max, c.Max())
	}

	tests := map[int]bool{
		0:   true,
		1:   true,
		2:   true,
		3:   true,
		5:   true,
		7:   true,
		29:  true,
		30:  true,
		31:  true,
		100: true,
		101: false,
		200: false,
	}

	runTests(t, c, tests)
}

func TestFilter(t *testing.T) {
	c := version.Filter(
		version.Min(3),
		version.Max(80),
		version.InRange(
			version.Range{20, 30},
			version.Range{100, 110},
		),
	)

	tests := map[int]bool{
		0:   false,
		2:   false,
		3:   false,
		19:  false,
		20:  true,
		25:  true,
		30:  true,
		31:  false,
		90:  false,
		100: false,
		110: false,
	}

	runTests(t, c, tests)
}

func runTests(t *testing.T, c version.Constraints, tests map[int]bool) {
	for give, want := range tests {
		if got := version.Includes(c, give); got != want {
			t.Errorf("expected version.Includes(c, %d) to return %t; got %t", give, want, got)
		}
	}
}
