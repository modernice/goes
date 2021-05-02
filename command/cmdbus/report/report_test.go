package report_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/command/cmdbus/report"
)

type mockPayload struct{}

func TestReport_Report(t *testing.T) {
	var r report.Report

	cmd := report.Command{
		Name:          "foo",
		ID:            uuid.New(),
		AggregateName: "foo",
		AggregateID:   uuid.New(),
	}
	err := errors.New("mock error")
	d := 10 * time.Second

	want := report.New(cmd, report.Runtime(d), report.Error(err))

	r.Report(want)

	if r.Command != cmd {
		t.Errorf("r.Command() should return %v; got %v", cmd, r.Command)
	}

	if r.Runtime != d {
		t.Errorf("r.Runtime() should return %v; got %v", r.Runtime, d)
	}

	if r.Error != err {
		t.Errorf("r.Err() should return %v; got %v", err, r.Error)
	}
}
