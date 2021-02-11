package dispatch

// Config is the configuration for dispatching a Command.
type Config struct {
	// A synchronous dispatch waits for the execution of the Command to finish
	// and returns the execution error if there was any.
	Synchronous bool
}

// Option is a dispatch option.
type Option func(*Config)

// Configure returns a Config from Options.
func Configure(opts ...Option) Config {
	var cfg Config
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}

// Synchronous returns an Option that makes dispatches synchronous.
// A synchronous dispatch also returns errors that happen during the execution
// of a Command.
func Synchronous() Option {
	return func(cfg *Config) {
		cfg.Synchronous = true
	}
}
