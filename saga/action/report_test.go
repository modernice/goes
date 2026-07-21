package action_test

import (
	"errors"
	"testing"
	"time"

	"github.com/modernice/goes/internal/xtime"
	"github.com/modernice/goes/saga/action"
)

func TestReport(t *testing.T) {
	start := xtime.Now()
	dur := 12345678 * time.Millisecond
	end := start.Add(dur)

	act := action.New("foo", nil)
	r := action.NewReport(act, start, end)

	if r.Action != act {
		t.Errorf("Action() should return %v; got %v", act, r.Action)
	}

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
	act := action.New("foo", nil)
	r := action.NewReport(act, xtime.Now(), xtime.Now(), action.Error(mockError))

	if r.Error != mockError {
		t.Errorf("Error() should return %q; got %q", mockError, r.Error)
	}
}

func TestCompensatedBy(t *testing.T) {
	act := action.New("foo", nil)
	comp := action.New("bar", nil)
	compRep := action.NewReport(comp, xtime.Now(), xtime.Now())
	r := action.NewReport(act, xtime.Now(), xtime.Now(), action.CompensatedBy(compRep))

	if *r.Compensator != compRep {
		t.Errorf("Compensator() should return %v; got %v", compRep, r.Compensator)
	}
}
