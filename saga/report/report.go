package report

import (
	"time"

	"github.com/modernice/goes/saga/action"
)

// A Report gives information about the execution of a SAGA.
type Report[E, C any] struct {
	// Start is the Time the SAGA started.
	Start time.Time
	// End is the Time the SAGA finished.
	End time.Time
	// Runtime is the runtime of the SAGA.
	Runtime time.Duration
	// Actions are the Actions that have been run.
	Actions []action.Report[E, C]
	// Succeeded are the succeeded Actions.
	Succeeded []action.Report[E, C]
	// Failed are the failed Actions.
	Failed []action.Report[E, C]
	// Compensated are the compensated Actions.
	Compensated []action.Report[E, C]
	// Error is the error that made the SAGA fail.
	Error error
}

// An Option adds information to a Report.
type Option[E, C any] func(*Report[E, C])

// Action returns an Option that adds an action.Report[E, C] to a SAGA Report.
func Action[E, C any](
	act action.Action[E, C],
	start, end time.Time,
	opts ...action.Option[E, C],
) Option[E, C] {
	return Add(action.NewReport(act, start, end, opts...))
}

// Add returns an Option that adds action.Reports[E, C] to a SAGA Report.
func Add[E, C any](acts ...action.Report[E, C]) Option[E, C] {
	return func(r *Report[E, C]) {
		r.Actions = append(r.Actions, acts...)
	}
}

// Error returns an Option that adds an error to a Report.
func Error[E, C any](err error) Option[E, C] {
	return func(r *Report[E, C]) {
		r.Error = err
	}
}

// New returns a Report that is filled by opts.
func New[E, C any](start, end time.Time, opts ...Option[E, C]) Report[E, C] {
	return new(start, end, opts...)
}

func new[E, C any](start, end time.Time, opts ...Option[E, C]) Report[E, C] {
	r := Report[E, C]{
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
func (r *Report[E, C]) Report(r2 Report[E, C]) {
	*r = r2
}

func (r *Report[E, C]) groupActions() {
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
