package dispatch

import "github.com/modernice/goes/command"

// Configure returns a Config from Options.
func Configure(opts ...command.DispatchOption) command.DispatchConfig {
	var cfg command.DispatchConfig
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
func Synchronous() command.DispatchOption {
	return func(cfg *command.DispatchConfig) {
		cfg.Synchronous = true
	}
}

// Report returns an Option that makes the Command Bus report the executon
// result of a Command to the Reporter r.
func Report(r command.Reporter) command.DispatchOption {
	return func(cfg *command.DispatchConfig) {
		cfg.Reporter = r
	}
}
