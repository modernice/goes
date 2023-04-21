package eventbustest

import (
	"testing"

	"github.com/modernice/goes/event"
)

// Option is a function type that modifies the configuration of an event bus
// test. It takes a pointer to a config struct as its argument and modifies its
// fields. Option functions are used to configure event bus tests with custom
// options. The Cleanup function is one such option that takes a testing.T
// instance and an event.Bus instance to clean up the event bus after a test is
// completed.
type Option func(*config)

type config struct {
	cleanup func(event.Bus) error
}

// Cleanup returns an Option that sets a function to clean up an event bus after
// testing is complete. The function takes an event.Bus as input and returns an
// error. This Option is used in the configure function to set up a
// configuration for testing, and in the Cleanup method to execute the cleanup
// function with the provided event bus. If the cleanup function returns an
// error, Cleanup will call t.Fatalf with a formatted error message.
func Cleanup[Bus event.Bus](cleanup func(Bus) error) Option {
	return func(cfg *config) {
		cfg.cleanup = func(bus event.Bus) error {
			return cleanup(bus.(Bus))
		}
	}
}

func configure(opts ...Option) config {
	var cfg config
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}

// Cleanup is a function that returns an Option to configure the Cleanup
// function of an event bus. The Cleanup function is called at the end of each
// test and is given the event bus used in the test as an argument. If an error
// occurs during cleanup, Cleanup will cause the test to fail with a fatal
// error.
func (cfg config) Cleanup(t *testing.T, bus event.Bus) {
	if cfg.cleanup != nil {
		if err := cfg.cleanup(bus); err != nil {
			t.Fatalf("event bus cleanup: %v", err)
		}
	}
}
