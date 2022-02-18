package report_test

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/internal/xtime"
	"github.com/modernice/goes/saga/action"
	"github.com/modernice/goes/saga/report"
)

func TestReport(t *testing.T) {
	start := xtime.Now()
	dur := 12345678 * time.Millisecond
	end := start.Add(dur)
	r := report.New[uuid.UUID](start, end)

	if !r.Start.Equal(start) {
		t.Errorf("Start() should return %v; got %v", start, r.Start)
	}

	if !r.End.Equal(end) {
		t.Errorf("End() should return %v; got %v", end, r.End)
	}

	if r.Runtime != dur {
		t.Errorf("Runtime() should return %v; got %v", dur, r.Runtime)
	}
}

func TestError(t *testing.T) {
	mockError := errors.New("mock error")
	r := report.New[uuid.UUID](xtime.Now(), xtime.Now(), report.Error[uuid.UUID](mockError))

	if r.Error != mockError {
		t.Errorf("Error() should return %q; got %q", mockError, r.Error)
	}
}

func TestAction(t *testing.T) {
	start := xtime.Now()
	dur := 12345678 * time.Millisecond
	end := start.Add(dur)
	act := action.New[uuid.UUID]("foo", nil)
	mockError := errors.New("mock error")
	r := report.New[uuid.UUID](xtime.Now(), xtime.Now(), report.Action(act, start, end, action.Error[uuid.UUID](mockError)))

	acts := r.Actions
	if len(acts) != 1 {
		t.Fatalf("r.Actions() should return %d Actions; got %d", 1, len(acts))
	}

	rep := acts[0]
	if rep.Action != act {
		t.Errorf("rep.Action() should return %v; got %v", act, rep.Action)
	}

	if !rep.Start.Equal(start) {
		t.Errorf("rep.Start() should return %v; got %v", start, rep.Start)
	}

	if !rep.End.Equal(end) {
		t.Errorf("rep.End() should return %v; got %v", end, rep.End)
	}

	if rep.Runtime != dur {
		t.Errorf("rep.Runtime() should return %v; got %v", dur, rep.Runtime)
	}

	if rep.Error != mockError {
		t.Errorf("rep.Error() should return %q; got %q", mockError, rep.Error)
	}
}

func TestAction_grouping(t *testing.T) {
	mockError := errors.New("mock error")
	compRep := action.NewReport(action.New[uuid.UUID]("comp1", nil), xtime.Now(), xtime.Now())
	compErrorRep := action.NewReport(action.New[uuid.UUID]("comp2", nil), xtime.Now(), xtime.Now(), action.Error[uuid.UUID](mockError))
	opts := []report.Option[uuid.UUID]{
		report.Action(action.New[uuid.UUID]("foo", nil), xtime.Now(), xtime.Now()),
		report.Action(action.New[uuid.UUID]("bar", nil), xtime.Now(), xtime.Now()),
		report.Action(action.New[uuid.UUID]("baz", nil), xtime.Now(), xtime.Now(), action.Error[uuid.UUID](mockError)),
		report.Action(action.New[uuid.UUID]("foobar", nil), xtime.Now(), xtime.Now(), action.Error[uuid.UUID](mockError)),
		report.Action(action.New[uuid.UUID]("barbaz", nil), xtime.Now(), xtime.Now(), action.Error[uuid.UUID](mockError), action.CompensatedBy(compRep)),
		report.Action(action.New[uuid.UUID]("bazfoo", nil), xtime.Now(), xtime.Now(), action.Error[uuid.UUID](mockError), action.CompensatedBy(compErrorRep)),
	}
	r := report.New[uuid.UUID](xtime.Now(), xtime.Now(), opts...)

	wantSucceeded := []string{"foo", "bar"}
	wantFailed := []string{"baz", "foobar", "barbaz", "bazfoo"}
	wantCompensated := []string{"barbaz", "bazfoo"}

	succeeded := r.Succeeded
	for i, want := range wantSucceeded {
		got := succeeded[i]
		if got.Action.Name() != want {
			t.Errorf("Succeeded()[%d].Action().Name() should return %q; got %q", i, want, got.Action.Name())
		}
	}

	failed := r.Failed
	for i, want := range wantFailed {
		got := failed[i]
		if got.Action.Name() != want {
			t.Errorf("Failed()[%d].Action().Name() should return %q; got %q", i, want, got.Action.Name())
		}
	}

	compensated := r.Compensated
	for i, want := range wantCompensated {
		got := compensated[i]
		if got.Action.Name() != want {
			t.Errorf("Compensated()[%d].Action().Name() should return %q; got %q", i, want, got.Action.Name())
		}
	}
}

func TestReport_Report(t *testing.T) {
	mockError := errors.New("mock error")
	rep := report.New(
		xtime.Now(), xtime.Now(),
		report.Action(action.New[uuid.UUID]("foo", nil), xtime.Now(), xtime.Now(), action.Error[uuid.UUID](mockError)),
		report.Error[uuid.UUID](mockError),
	)

	var r report.Report[uuid.UUID]
	r.Report(rep)

	if !reflect.DeepEqual(r, rep) {
		t.Errorf("filled Report should match source Report.\n\nsource: %#v\n\nfilled: %#v", rep, r)
	}
}
