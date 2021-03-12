package report_test

import (
	"errors"
	"math/rand"
	"testing"
	"time"

	"github.com/modernice/goes/command"
	"github.com/modernice/goes/command/cmdbus/report"
)

type mockPayload struct{}

func TestRuntime(t *testing.T) {
	d := time.Duration(rand.Intn(10000)) * time.Millisecond
	r := report.New(nil, report.Runtime(d))

	if r.Runtime() != d {
		t.Fatalf("r.Runtime() should return %s; got %s", d, r.Runtime())
	}
}

func TestError(t *testing.T) {
	err := errors.New("mock error")
	r := report.New(nil, report.Error(err))

	if r.Err() != err {
		t.Fatalf("r.Err() should return %v; got %v", err, r.Err())
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

	if r.Err() != err {
		t.Errorf("r.Err() should return %v; got %v", err, r.Err())
	}
}
