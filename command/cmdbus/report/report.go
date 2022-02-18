package report

import (
	"time"

	"github.com/modernice/goes"
)

// A Report provides information about the execution of a Command.
type Report[ID goes.ID] struct {
	Command Command[ID]
	Runtime time.Duration
	Error   error
}

type Command[ID goes.ID] struct {
	ID            ID
	Name          string
	AggregateName string
	AggregateID   ID
	Payload       any
}

// Option is a Report option.
type Option[ID goes.ID] func(*Report[ID])

// New returns a new Report that is filled with the information from opts.
func New[ID goes.ID](cmd Command[ID], opts ...Option[ID]) Report[ID] {
	r := Report[ID]{Command: cmd}
	for _, opt := range opts {
		opt(&r)
	}
	return r
}

// Runtime returns an Option that specifies the runtime of a Command execution.
func Runtime[ID goes.ID](d time.Duration) Option[ID] {
	return func(r *Report[ID]) {
		r.Runtime = d
	}
}

// Error returns a ReportOption that adds the execution error of a Command to a
// Report.
func Error[ID goes.ID](err error) Option[ID] {
	return func(r *Report[ID]) {
		r.Error = err
	}
}

func (r *Report[ID]) Report(rep Report[ID]) {
	*r = New(rep.Command, Runtime[ID](rep.Runtime), Error[ID](rep.Error))
}
