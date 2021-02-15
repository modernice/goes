package report

import (
	"time"

	"github.com/modernice/goes/saga/action"
)

// A Report gives information about the execution of a SAGA.
type Report struct {
	start, end  time.Time
	runtime     time.Duration
	acts        []action.Report
	succeeded   []action.Report
	failed      []action.Report
	compensated []action.Report
	err         error
}

// An Option adds information to a Report.
type Option func(*Report)

// Action returns an Option that adds an action.Report to a SAGA Report.
func Action(
	act action.Action,
	start, end time.Time,
	opts ...action.ReportOption,
) Option {
	return Add(action.NewReport(act, start, end, opts...))
}

// Add returns an Option that adds action.Reports to a SAGA Report.
func Add(acts ...action.Report) Option {
	return func(r *Report) {
		r.acts = append(r.acts, acts...)
	}
}

// Error returns an Option that adds an error to a Report.
func Error(err error) Option {
	return func(r *Report) {
		r.err = err
	}
}

// New returns a Report that is filled by opts.
func New(start, end time.Time, opts ...Option) Report {
	return new(start, end, opts...)
}

func new(start, end time.Time, opts ...Option) Report {
	r := Report{
		start:   start,
		end:     end,
		runtime: end.Sub(start),
	}
	for _, opt := range opts {
		opt(&r)
	}
	r.groupActions()
	return r
}

// Start returns the start Time of the SAGA.
func (r Report) Start() time.Time {
	return r.start
}

// End returns the end Time of the SAGA.
func (r Report) End() time.Time {
	return r.end
}

// Runtime returns the runtime of the SAGA.
func (r Report) Runtime() time.Duration {
	return r.runtime
}

// Error returns the error that made the SAGA fail. It returns the same error
// that is returned from the Execute function of a SAGA:
//
//	var r report.Report
//	s := saga.New(saga.Report(&r))
//	err := s.Execute(context.TODO())
//	// err == r.Error()
func (r Report) Error() error {
	return r.err
}

// Actions returns the action.Reports for Actions that ran during the SAGA.
func (r Report) Actions() []action.Report {
	return r.acts
}

// Succeeded returns the action.Reports for the succeeded Actions of the SAGA.
func (r Report) Succeeded() []action.Report {
	return r.succeeded
}

// Failed returns the action.Reports for the failed Actions of the SAGA.
func (r Report) Failed() []action.Report {
	return r.failed
}

// Compensated returns the action.Reports for the compensated Actions of the
// SAGA. Compensated returns a subset of failed. This means that every
// action.Report that is returned from Compensated is also returned from Failed,
// but not every Report that is returned from Failed is also returned from
// Compensated.
func (r Report) Compensated() []action.Report {
	return r.compensated
}

// Report overrides Report r with the contents of Report r2.
func (r *Report) Report(r2 Report) {
	*r = r2
}

func (r *Report) groupActions() {
	for _, rep := range r.acts {
		if rep.Error() != nil {
			r.failed = append(r.failed, rep)
			if rep.Compensator() != nil {
				r.compensated = append(r.compensated, rep)
			}
			continue
		}
		r.succeeded = append(r.succeeded, rep)
	}
}
