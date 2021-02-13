package done_test

import (
	"errors"
	"math/rand"
	"testing"
	"time"

	"github.com/modernice/goes/command/done"
)

func TestWithError(t *testing.T) {
	mockError := errors.New("mock error")
	cfg := done.Configure(done.WithError(mockError))

	if cfg.Err != mockError {
		t.Fatalf("cfg.Err should be %v; got %v", mockError, cfg.Err)
	}
}

func TestRuntime(t *testing.T) {
	dur := time.Duration(rand.Intn(100)) * time.Second
	cfg := done.Configure(done.WithRuntime(dur))

	if cfg.Runtime != dur {
		t.Fatalf("cfg.Runtime should be %s; got %s", dur, cfg.Runtime)
	}
}
