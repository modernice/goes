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

// New creates a Report for the given Command.
func New(cmd command.Command, opts ...Option) Report {
	r := Report{cmd: cmd}
	for _, opt := range opts {
		opt(&r)
	}
	return r
}

// Command returns the Command.
func (r Report) Command() command.Command {
	return r.cmd
}

// Runtime returns the runtime of the Command execution.
func (r Report) Runtime() time.Duration {
	return r.runtime
}

// Error returns the Command execution error.
func (r Report) Error() error {
	return r.err
}
