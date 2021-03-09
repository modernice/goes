package dispatch

import "github.com/modernice/goes/command/cmdbus/report"

// Config is the configuration for dispatching a Command.
type Config struct {
	// A synchronous dispatch waits for the execution of the Command to finish
	// and returns the execution error if there was any.
	//
	// A dispatch is automatically made synchronous when Repoter is non-nil.
	Synchronous bool

	// If Reporter is not nil, the Bus will report the execution result of a
	// Command to Reporter by calling Reporter.Report().
	//
	// A non-nil Reporter makes the dispatch synchronous.
	Reporter Reporter
}

// A Reporter reports execution results of Commands.
type Reporter interface {
	// Report reports the execution result of a Command.
	Report(report.Report)
}

// Option is a dispatch option.
type Option func(*Config)

// Configure returns a Config from Options.
func Configure(opts ...Option) Config {
	var cfg Config
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.Reporter != nil {
		cfg.Synchronous = true
	}
	return cfg
}

// Synchronous returns an Option that makes dispatches synchronous.
//
// A synchronous dispatch also returns errors that happen during the execution
// of a Command.
func Synchronous() Option {
	return func(cfg *Config) {
		cfg.Synchronous = true
	}
}

// Report returns an Option that makes the Command Bus report the executon
// result of a Command to the Reporter r.
func Report(r Reporter) Option {
	return func(cfg *Config) {
		cfg.Reporter = r
	}
}
