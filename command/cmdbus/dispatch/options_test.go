package dispatch_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/modernice/goes/command/cmdbus/dispatch"
	"github.com/modernice/goes/command/cmdbus/report"
)

func TestSynchronous(t *testing.T) {
	cfg := dispatch.Configure(dispatch.Sync())
	if !cfg.Synchronous {
		t.Fatalf("cfg.Synchronous should be %t; got %t", true, cfg.Synchronous)
	}
}

func TestReport(t *testing.T) {
	var rep report.Report[uuid.UUID]
	cfg := dispatch.Configure(dispatch.Report[uuid.UUID](&rep))

	if cfg.Reporter != &rep {
		t.Fatalf("cfg.Report should point to %p; got %v", &rep, cfg.Reporter)
	}
}
