package done

import "time"

// Config is the configuration for reporting the execution result of a Command.
type Config struct {
	Err     error
	Runtime time.Duration
}

// Option is a done option
type Option func(*Config)

// Configure returns a Config from Options.
func Configure(opts ...Option) Config {
	var cfg Config
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}

// WithError returns an Option that adds reports the execution error of a
// Command.
func WithError(err error) Option {
	return func(cfg *Config) {
		cfg.Err = err
	}
}

// WithRuntime returns an Option that overrides the measured execution time of a
// Command.
func WithRuntime(d time.Duration) Option {
	return func(cfg *Config) {
		cfg.Runtime = d
	}
}
