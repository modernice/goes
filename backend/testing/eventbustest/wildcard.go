package eventbustest

import (
	"context"
	"testing"
	"time"

	"github.com/modernice/goes/event"
)

func RunWildcard(t *testing.T, newBus EventBusFactory, opts ...Option) {
	cfg := configure(opts...)

	t.Run("Wildcard", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		bus := newBus(enc)

		defer cfg.Cleanup(t, bus)

		str, errs, err := bus.Subscribe(ctx, event.All)
		if err != nil {
			t.Fatalf("subscribe to events: %v", err)
		}
		sub := Sub(str, errs)

		ex := Expect(ctx)
		ex.Events(sub, 500*time.Millisecond, "foo", "bar", "baz")

		events := []event.Event{
			event.New("foo", "foobar").Any(),
			event.New("bar", "foobar").Any(),
			event.New("baz", "foobar").Any(),
		}

		if err := bus.Publish(ctx, events...); err != nil {
			t.Fatalf("publish events: %v", err)
		}

		ex.Apply(t)
	})
}
