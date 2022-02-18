package action

import (
	"time"

	"github.com/modernice/goes"
)

// An Option adds information to a Report.
type Option[ID goes.ID] func(*Report[ID])

type Report[ID goes.ID] struct {
	Action      Action[ID]
	Start       time.Time
	End         time.Time
	Runtime     time.Duration
	Error       error
	Compensator *Report[ID]
}

// Error returns a Option that adds the error of an Action to a Report.
func Error[ID goes.ID](err error) Option[ID] {
	return func(r *Report[ID]) {
		r.Error = err
	}
}

// CompensatedBy returns a Option that adds the Report of a compensating
// Action to the Report of a failed Action.
func CompensatedBy[ID goes.ID](rep Report[ID]) Option[ID] {
	return func(r *Report[ID]) {
		r.Compensator = &rep
	}
}

// NewReport returns a Report for the given Action. The returned Report provides
// at least the Action and its start-, end- & runtime. Additional information
// can be added by providing Options.
func NewReport[ID goes.ID](act Action[ID], start, end time.Time, opts ...Option[ID]) Report[ID] {
	r := Report[ID]{
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
