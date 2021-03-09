package report

import (
	"time"

	"github.com/google/uuid"
)

// A Report provides information about the execution of a Command. A *Report is
// also a Reporter.
type Report struct {
	cmd     Command
	runtime time.Duration
	err     error
}

// Command is a subset of command.Command. Redeclared here to avoid import
// cycles.
type Command interface {
	// ID returns the UUID of the Command.
	ID() uuid.UUID

	// Name returns the name of the Command.
	Name() string
}

// Option is a Report option.
type Option func(*Report)

// New returns a new Report that is filled with the information from opts.
func New(cmd Command, opts ...Option) Report {
	r := Report{cmd: cmd}
	for _, opt := range opts {
		opt(&r)
	}
	return r
}

// Runtime returns an Option that specifies the runtime of a Command execution.
func Runtime(d time.Duration) Option {
	return func(r *Report) {
		r.runtime = d
	}
}

// Error returns an Option that specifies the execution error of a Command.
func Error(err error) Option {
	return func(r *Report) {
		r.err = err
	}
}

// Command returns the Command.
func (r Report) Command() Command {
	return r.cmd
}

// Runtime returns the runtime of the Command.
func (r Report) Runtime() time.Duration {
	return r.runtime
}

// Error returns the execution error of the Command.
func (r Report) Error() error {
	return r.err
}

// Report fills the Report r with the data in Report rep.
func (r *Report) Report(rep Report) {
	*r = New(rep.Command(), Runtime(rep.runtime), Error(rep.Error()))
}
