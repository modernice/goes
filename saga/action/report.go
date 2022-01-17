package action

import "time"

// An Option adds information to a Report.
type Option[E, C any] func(*Report[E, C])

type Report[E, C any] struct {
	Action      Action[E, C]
	Start       time.Time
	End         time.Time
	Runtime     time.Duration
	Error       error
	Compensator *Report[E, C]
}

// Error returns a Option that adds the error of an Action to a Report.
func Error[E, C any](err error) Option[E, C] {
	return func(r *Report[E, C]) {
		r.Error = err
	}
}

// CompensatedBy returns a Option that adds the Report of a compensating
// Action to the Report of a failed Action.
func CompensatedBy[E, C any](rep Report[E, C]) Option[E, C] {
	return func(r *Report[E, C]) {
		r.Compensator = &rep
	}
}

// NewReport returns a Report for the given Action. The returned Report provides
// at least the Action and its start-, end- & runtime. Additional information
// can be added by providing Options.
func NewReport[E, C any](act Action[E, C], start, end time.Time, opts ...Option[E, C]) Report[E, C] {
	r := Report[E, C]{
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
