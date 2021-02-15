package action

import "time"

// A Report provides information about the result of an Action.
type Report interface {
	// Action returns the Action.
	Action() Action

	// Start returns the start time of the Action.
	Start() time.Time

	// End returns the end time of the Action.
	End() time.Time

	// Runtime returns the runtime of the Action.
	Runtime() time.Duration

	// Error returns the error that made the Action fail.
	Error() error

	// Compensator returns the Report of the compensating Action for this
	// Action.
	Compensator() Report
}

// A ReportOption adds information to a Report.
type ReportOption func(*report)

type report struct {
	act         Action
	start, end  time.Time
	runtime     time.Duration
	err         error
	compensator Report
}

// Error returns a ReportOption that adds the error of an Action to a Report.
func Error(err error) ReportOption {
	return func(r *report) {
		r.err = err
	}
}

// CompensatedBy returns a ReportOption that adds the Report of a compensating
// Action to the Report of a failed Action.
func CompensatedBy(rep Report) ReportOption {
	return func(r *report) {
		r.compensator = rep
	}
}

// NewReport returns a Report for the given Action. The returned Report provides
// at least the Action and its start-, end- & runtime. Additional information
// can be added by providing ReportOptions.
func NewReport(act Action, start, end time.Time, opts ...ReportOption) Report {
	r := report{
		act:     act,
		start:   start,
		end:     end,
		runtime: end.Sub(start),
	}
	for _, opt := range opts {
		opt(&r)
	}
	return r
}

func (r report) Action() Action {
	return r.act
}

func (r report) Start() time.Time {
	return r.start
}

func (r report) End() time.Time {
	return r.end
}

func (r report) Runtime() time.Duration {
	return r.runtime
}

func (r report) Error() error {
	return r.err
}

func (r report) Compensator() Report {
	return r.compensator
}
