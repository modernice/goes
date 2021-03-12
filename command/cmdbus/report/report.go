package report

import (
	"time"

	"github.com/modernice/goes/command"
)

// A Report provides information about the execution of a Command.
type Report struct {
	cmd     command.Command
	runtime time.Duration
	err     error
}

// Option is a Report option.
type Option func(*Report)

// New returns a new Report that is filled with the information from opts.
func New(cmd command.Command, opts ...Option) Report {
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

// Error returns a ReportOption that adds the execution error of a Command to a
// Report.
func Error(err error) Option {
	return func(r *Report) {
		r.err = err
	}
}

func (r Report) Command() command.Command {
	return r.cmd
}

func (r Report) Runtime() time.Duration {
	return r.runtime
}

func (r Report) Err() error {
	return r.err
}

func (r *Report) Report(rep command.Report) {
	*r = New(rep.Command(), Runtime(rep.Runtime()), Error(rep.Err()))
}
