package report_test

import (
	"errors"
	"math/rand"
	"testing"
	"time"

	"github.com/modernice/goes/command"
	"github.com/modernice/goes/command/cmdbus/dispatch"
	"github.com/modernice/goes/command/cmdbus/report"
)

var _ dispatch.Reporter = &report.Report{}

type mockPayload struct{}

func TestRuntime(t *testing.T) {
	d := time.Duration(rand.Intn(10000)) * time.Millisecond
	var r report.Report
	report.Runtime(d)(&r)

	if r.Runtime() != d {
		t.Fatalf("r.Runtime() should return %s; got %s", d, r.Runtime())
	}
}

func TestError(t *testing.T) {
	err := errors.New("mock error")
	var r report.Report
	report.Error(err)(&r)

	if r.Error() != err {
		t.Fatalf("r.Error() should return %v; got %v", err, r.Error())
	}
}

func TestReport_Report(t *testing.T) {
	var r report.Report

	cmd := command.New("foo", mockPayload{})
	err := errors.New("mock error")
	d := 10 * time.Second

	want := report.New(cmd, report.Runtime(d), report.Error(err))

	r.Report(want)

	if r.Command() != cmd {
		t.Errorf("r.Command() should return %v; got %v", cmd, r.Command())
	}

	if r.Runtime() != d {
		t.Errorf("r.Runtime() should return %v; got %v", r.Runtime(), d)
	}

	if r.Error() != err {
		t.Errorf("r.Error() should return %v; got %v", err, r.Error())
	}
}
