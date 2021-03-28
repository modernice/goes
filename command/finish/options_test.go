package finish_test

import (
	"errors"
	"math/rand"
	"testing"
	"time"

	"github.com/modernice/goes/command/finish"
)

func TestWithError(t *testing.T) {
	mockError := errors.New("mock error")
	cfg := finish.Configure(finish.WithError(mockError))

	if cfg.Err != mockError {
		t.Fatalf("cfg.Err should be %v; got %v", mockError, cfg.Err)
	}
}

func TestRuntime(t *testing.T) {
	dur := time.Duration(rand.Intn(100)) * time.Second
	cfg := finish.Configure(finish.WithRuntime(dur))

	if cfg.Runtime != dur {
		t.Fatalf("cfg.Runtime should be %s; got %s", dur, cfg.Runtime)
	}
}
