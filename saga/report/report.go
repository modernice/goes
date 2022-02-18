package report

import (
	"time"

	"github.com/modernice/goes"
	"github.com/modernice/goes/saga/action"
)

// A Report gives information about the execution of a SAGA.
type Report[ID goes.ID] struct {
	// Start is the Time the SAGA started.
	Start time.Time
	// End is the Time the SAGA finished.
	End time.Time
	// Runtime is the runtime of the SAGA.
	Runtime time.Duration
	// Actions are the Actions that have been run.
	Actions []action.Report[ID]
	// Succeeded are the succeeded Actions.
	Succeeded []action.Report[ID]
	// Failed are the failed Actions.
	Failed []action.Report[ID]
	// Compensated are the compensated Actions.
	Compensated []action.Report[ID]
	// Error is the error that made the SAGA fail.
	Error error
}

// An Option adds information to a Report.
type Option[ID goes.ID] func(*Report[ID])

// Action returns an Option that adds an action.Report to a SAGA Report.
func Action[ID goes.ID](
	act action.Action[ID],
	start, end time.Time,
	opts ...action.Option[ID],
) Option[ID] {
	return Add(action.NewReport(act, start, end, opts...))
}

// Add returns an Option that adds action.Reports to a SAGA Report.
func Add[ID goes.ID](acts ...action.Report[ID]) Option[ID] {
	return func(r *Report[ID]) {
		r.Actions = append(r.Actions, acts...)
	}
}

// Error returns an Option that adds an error to a Report.
func Error[ID goes.ID](err error) Option[ID] {
	return func(r *Report[ID]) {
		r.Error = err
	}
}

// New returns a Report that is filled by opts.
func New[ID goes.ID](start, end time.Time, opts ...Option[ID]) Report[ID] {
	return newReport(start, end, opts...)
}

func newReport[ID goes.ID](start, end time.Time, opts ...Option[ID]) Report[ID] {
	r := Report[ID]{
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
func (r *Report[ID]) Report(r2 Report[ID]) {
	*r = r2
}

func (r *Report[ID]) groupActions() {
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
