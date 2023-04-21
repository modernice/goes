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

// Command represents a command to be executed in a system. It contains an ID,
// name, aggregate name, aggregate ID, and payload. A Report provides
// information about the execution of a Command, including the Command itself,
// runtime duration, and any errors encountered during execution. New creates a
// new Report with the given Command and options. Options include Runtime to
// specify the runtime of a Command execution and Error to add the execution
// error of a Command to a Report.
type Command struct {
	ID            uuid.UUID
	Name          string
	AggregateName string
	AggregateID   uuid.UUID
	Payload       any
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

// Report.Report updates the Report instance with the information from the
// provided Report instance. It creates a new Report based on the Command in the
// provided Report, and updates the runtime and error information. This method
// is useful for aggregating multiple Reports into a single Report.
func (r *Report) Report(rep Report) {
	*r = New(rep.Command, Runtime(rep.Runtime), Error(rep.Error))
}
