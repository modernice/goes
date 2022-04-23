package eventbustest

import (
	"testing"

	"github.com/modernice/goes/event"
)

type Option func(*config)

type config struct {
	cleanup func(event.Bus) error
}

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

func (cfg config) Cleanup(t *testing.T, bus event.Bus) {
	if cfg.cleanup != nil {
		if err := cfg.cleanup(bus); err != nil {
			t.Fatalf("event bus cleanup: %v", err)
		}
	}
}
