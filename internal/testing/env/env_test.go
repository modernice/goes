package env_test

import (
	"os"
	"testing"
	"time"

	"github.com/modernice/goes/internal/testing/env"
)

func TestTemp(t *testing.T) {
	if err := os.Setenv("foo", "foo"); err != nil {
		t.Fatal(err)
	}

	restore := env.Temp("foo", "bar")

	if val := os.Getenv("foo"); val != "bar" {
		t.Fatalf("expected os.Getenv(%q) to return %q; got %q", "foo", "bar", val)
	}

	restore()

	if val := os.Getenv("foo"); val != "foo" {
		t.Fatalf("expected os.Getenv(%q) to return %q; got %q", "foo", "foo", val)
	}
}

func TestBool(t *testing.T) {
	tests := map[string]bool{
		"1":     true,
		"0":     false,
		"true":  true,
		"false": false,
		"TRUE":  true,
		"FALSE": false,
		"TruE":  true,
		"fAlSe": false,
		"yes":   true,
		"y":     true,
		"no":    false,
		"n":     false,
		"on":    true,
		"off":   false,
		"ofF":   false,
		"ok":    true,
		"":      false,
		" ":     false,
	}

	for give, want := range tests {
		restore := env.Temp("foo", give)
		if val := env.Bool("foo"); val != want {
			t.Errorf("expected %q to be %t", give, want)
		}
		restore()
	}
}

func TestInt(t *testing.T) {
	tests := map[string]int{
		"0":         0,
		"1":         1,
		"123456789": 123456789,
		"-99999999": -99999999,
	}

	for give, want := range tests {
		restore := env.Temp("foo", give)
		if val, _ := env.Int("foo"); val != want {
			t.Errorf("expected %q to be %d", "foo", want)
		}
		restore()
	}
}

func TestDuration(t *testing.T) {
	tests := map[string]time.Duration{
		"10s":   10 * time.Second,
		"250ms": 250 * time.Millisecond,
		"2m5s120ms30us700ns": 2*time.Minute + 5*time.Second +
			120*time.Millisecond + 30*time.Microsecond + 700*time.Nanosecond,
	}

	for give, want := range tests {
		restore := env.Temp("foo", give)
		if d, _ := env.Duration("foo"); d != want {
			t.Errorf("expected %q to equal %s; got %s", give, want, d)
		}
		restore()
	}
}
