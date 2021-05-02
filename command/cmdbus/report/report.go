package report

import (
	"time"

	"github.com/google/uuid"
)

// A Report provides information about the execution of a Command.
type Report struct {
	Command Command
	Runtime time.Duration
	Error   error
}

type Command struct {
	ID            uuid.UUID
	Name          string
	AggregateName string
	AggregateID   uuid.UUID
	Payload       interface{}
}

// Option is a Report option.
type Option func(*Report)

// New returns a new Report that is filled with the information from opts.
func New(cmd Command, opts ...Option) Report {
	r := Report{Command: cmd}
	for _, opt := range opts {
		opt(&r)
	}
	return r
}

// Runtime returns an Option that specifies the runtime of a Command execution.
func Runtime(d time.Duration) Option {
	return func(r *Report) {
		r.Runtime = d
	}
}

// Error returns a ReportOption that adds the execution error of a Command to a
// Report.
func Error(err error) Option {
	return func(r *Report) {
		r.Error = err
	}
}

func (r *Report) Report(rep Report) {
	*r = New(rep.Command, Runtime(rep.Runtime), Error(rep.Error))
}
