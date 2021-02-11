package dispatch_test

import (
	"testing"

	"github.com/modernice/goes/command/cmdbus/dispatch"
)

func TestSynchronous(t *testing.T) {
	cfg := dispatch.Configure(dispatch.Synchronous())
	if !cfg.Synchronous {
		t.Fatalf("cfg.Sync should be %t; got %t", true, cfg.Synchronous)
	}
}
