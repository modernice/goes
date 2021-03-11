package done

import "time"

// Config is the configuration for reporting the execution result of a Command.
type Config struct {
	Err     error
	Runtime time.Duration
}

// Option is a Config option
type Option func(*Config)

// Configure returns a Config from the provided Options.
func Configure(opts ...Option) Config {
	var cfg Config
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}

// WithError returns an Option that adds an error to a Config.
func WithError(err error) Option {
	return func(cfg *Config) {
		cfg.Err = err
	}
}

// WithRuntime returns an Option that adds the execution runtime to a Config.
func WithRuntime(d time.Duration) Option {
	return func(cfg *Config) {
		cfg.Runtime = d
	}
}
