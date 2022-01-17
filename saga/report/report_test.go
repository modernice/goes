package report_test

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/modernice/goes/internal/xtime"
	"github.com/modernice/goes/saga/action"
	"github.com/modernice/goes/saga/report"
)

func TestReport(t *testing.T) {
	start := xtime.Now()
	dur := 12345678 * time.Millisecond
	end := start.Add(dur)
	r := report.New[any, any](start, end)

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
	r := report.New[any, any](xtime.Now(), xtime.Now(), report.Error[any, any](mockError))

	if r.Error != mockError {
		t.Errorf("Error() should return %q; got %q", mockError, r.Error)
	}
}

func TestAction(t *testing.T) {
	start := xtime.Now()
	dur := 12345678 * time.Millisecond
	end := start.Add(dur)
	act := action.New[any, any]("foo", nil)
	mockError := errors.New("mock error")
	r := report.New[any, any](xtime.Now(), xtime.Now(), report.Action(act, start, end, action.Error[any, any](mockError)))

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
	compRep := action.NewReport(action.New[any, any]("comp1", nil), xtime.Now(), xtime.Now())
	compErrorRep := action.NewReport(action.New[any, any]("comp2", nil), xtime.Now(), xtime.Now(), action.Error[any, any](mockError))
	opts := []report.Option[any, any]{
		report.Action(action.New[any, any]("foo", nil), xtime.Now(), xtime.Now()),
		report.Action(action.New[any, any]("bar", nil), xtime.Now(), xtime.Now()),
		report.Action(action.New[any, any]("baz", nil), xtime.Now(), xtime.Now(), action.Error[any, any](mockError)),
		report.Action(action.New[any, any]("foobar", nil), xtime.Now(), xtime.Now(), action.Error[any, any](mockError)),
		report.Action(action.New[any, any]("barbaz", nil), xtime.Now(), xtime.Now(), action.Error[any, any](mockError), action.CompensatedBy(compRep)),
		report.Action(action.New[any, any]("bazfoo", nil), xtime.Now(), xtime.Now(), action.Error[any, any](mockError), action.CompensatedBy(compErrorRep)),
	}
	r := report.New[any, any](xtime.Now(), xtime.Now(), opts...)

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
		report.Action(action.New[any, any]("foo", nil), xtime.Now(), xtime.Now(), action.Error[any, any](mockError)),
		report.Error[any, any](mockError),
	)

	var r report.Report[any, any]
	r.Report(rep)

	if !reflect.DeepEqual(r, rep) {
		t.Errorf("filled Report should match source Report.\n\nsource: %#v\n\nfilled: %#v", rep, r)
	}
}
