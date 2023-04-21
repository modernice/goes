package action

import "time"

// An Option adds information to a Report.
type Option func(*Report)

// Report represents a report of an action. It contains information about the 
// action, including start and end time, runtime, error (if any), and 
// compensator (if any). Additional information can be added using Options. Use 
// NewReport to create a new Report.
type Report struct {
	Action      Action
	Start       time.Time
	End         time.Time
	Runtime     time.Duration
	Error       error
	Compensator *Report
}

// Error returns a Option that adds the error of an Action to a Report.
func Error(err error) Option {
	return func(r *Report) {
		r.Error = err
	}
}

// CompensatedBy returns a Option that adds the Report of a compensating
// Action to the Report of a failed Action.
func CompensatedBy(rep Report) Option {
	return func(r *Report) {
		r.Compensator = &rep
	}
}

// NewReport returns a Report for the given Action. The returned Report provides
// at least the Action and its start-, end- & runtime. Additional information
// can be added by providing Options.
func NewReport(act Action, start, end time.Time, opts ...Option) Report {
	r := Report{
		Action:  act,
		Start:   start,
		End:     end,
		Runtime: end.Sub(start),
	}
	for _, opt := range opts {
		opt(&r)
	}
	return r
}
