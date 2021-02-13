package report_test

import (
	"errors"
	"math/rand"
	"testing"
	"time"

	"github.com/modernice/goes/command"
	"github.com/modernice/goes/command/cmdbus"
	"github.com/modernice/goes/command/cmdbus/report"
)

type mockPayload struct{}

func TestNew(t *testing.T) {
	cmd := command.New("foo", mockPayload{})
	r := report.New(cmd)

	var _ cmdbus.Report = r

	if r.Command() != cmd {
		t.Fatalf("r.Command() should return %v; got %v", cmd, r.Command())
	}

	if r.Runtime() != time.Duration(0) {
		t.Fatalf("r.Runtime() should return %s; got %s", time.Duration(0), r.Runtime())
	}

	if r.Error() != nil {
		t.Fatalf("r.Error() should return <nil>; got %v", r.Error())
	}
}

func TestRuntime(t *testing.T) {
	cmd := command.New("foo", mockPayload{})
	d := time.Duration(rand.Intn(10000)) * time.Millisecond
	r := report.New(cmd, report.Runtime(d))

	if r.Runtime() != d {
		t.Fatalf("r.Runtime() should return %s; got %s", d, r.Runtime())
	}
}

func TestError(t *testing.T) {
	cmd := command.New("foo", mockPayload{})
	err := errors.New("mock error")
	r := report.New(cmd, report.Error(err))

	if r.Error() != err {
		t.Fatalf("r.Error() should return %v; got %v", err, r.Error())
	}
}
