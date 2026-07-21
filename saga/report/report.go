package report

import (
	"time"

	"github.com/modernice/goes/saga/action"
)

// A Report gives information about the execution of a SAGA.
type Report struct {
	// Start is the Time the SAGA started.
	Start time.Time
	// End is the Time the SAGA finished.
	End time.Time
	// Runtime is the runtime of the SAGA.
	Runtime time.Duration
	// Actions are the Actions that have been run.
	Actions []action.Report
	// Succeeded are the succeeded Actions.
	Succeeded []action.Report
	// Failed are the failed Actions.
	Failed []action.Report
	// Compensated are the compensated Actions.
	Compensated []action.Report
	// Error is the error that made the SAGA fail.
	Error error
}

// An Option adds information to a Report.
type Option func(*Report)

// Action returns an Option that adds an action.Report to a SAGA Report.
func Action(
	act action.Action,
	start, end time.Time,
	opts ...action.Option,
) Option {
	return Add(action.NewReport(act, start, end, opts...))
}

// Add returns an Option that adds action.Reports to a SAGA Report.
func Add(acts ...action.Report) Option {
	return func(r *Report) {
		r.Actions = append(r.Actions, acts...)
	}
}

// Error returns an Option that adds an error to a Report.
func Error(err error) Option {
	return func(r *Report) {
		r.Error = err
	}
}

// New returns a Report that is filled by opts.
func New(start, end time.Time, opts ...Option) Report {
	return new(start, end, opts...)
}

func new(start, end time.Time, opts ...Option) Report {
	r := Report{
		Start:   start,
		End:     end,
		Runtime: end.Sub(start),
	}
	for _, opt := range opts {
		opt(&r)
	}
	r.groupActions()
	return r
}

// Report overrides Report r with the contents of Report r2.
func (r *Report) Report(r2 Report) {
	*r = r2
}

func (r *Report) groupActions() {
	for _, rep := range r.Actions {
		if rep.Error != nil {
			r.Failed = append(r.Failed, rep)
			if rep.Compensator != nil {
				r.Compensated = append(r.Compensated, rep)
			}
			continue
		}
		r.Succeeded = append(r.Succeeded, rep)
	}
}
