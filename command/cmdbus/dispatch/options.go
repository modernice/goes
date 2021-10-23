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

// Synchronous returns a DispatchOption that makes the dispatch of the command
// synchronous.
//
// A synchronous dispatch returns a potential error that occured during the
// execution of a command back to the dispatcher of the command.
//
// Deprecated: Use Sync instead.
func Synchronous() command.DispatchOption {
	return Sync()
}

// Sync returns a DispatchOption that makes the dispatch of the command
// synchronous.
//
// A synchronous dispatch returns a potential error that occured during the
// execution of a command back to the dispatcher of the command.
func Sync() command.DispatchOption {
	return func(cfg *command.DispatchConfig) {
		cfg.Synchronous = true
	}
}

// Async returns a DispatchOption that makes the dispatch of a command
// asynchronous if it has been previously made synchronous by the
// Sync/Synchronous option.
func Async() command.DispatchOption {
	return func(cfg *command.DispatchConfig) {
		cfg.Synchronous = false
	}
}

// Report returns an Option that makes the Command Bus report the executon
// result of a Command to the Reporter r.
func Report(r command.Reporter) command.DispatchOption {
	return func(cfg *command.DispatchConfig) {
		cfg.Reporter = r
	}
}
