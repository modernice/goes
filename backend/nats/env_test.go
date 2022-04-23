//go:build nats

package nats

import (
	"testing"

	"github.com/modernice/goes/event/test"
	"github.com/modernice/goes/internal/env"
	"github.com/nats-io/nats.go"
)

func TestEventBus_envNATSURL(t *testing.T) {
	recover := env.Temp("NATS_URL", "")
	defer recover()

	bus := NewEventBus(test.NewEncoder())
	if url := bus.natsURL(); url != nats.DefaultURL {
		t.Fatalf("bus.natsURL() should return %q; got %q", nats.DefaultURL, url)
	}

	want := "foo://bar:123"
	defer env.Temp("NATS_URL", want)()

	bus = NewEventBus(test.NewEncoder())
	if url := bus.natsURL(); url != want {
		t.Fatalf("bus.natsURL() should return %q; got %q", want, url)
	}
}
