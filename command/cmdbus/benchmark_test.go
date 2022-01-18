package cmdbus_test

import (
	"context"
	"log"
	"testing"
	"time"

	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/command"
	"github.com/modernice/goes/command/cmdbus/dispatch"
)

func BenchmarkBus_Dispatch_Synchronous(t *testing.B) {
	bus, _, enc := newBus()
	codec.Gob(enc).GobRegister("foo", func() any { return struct{}{} })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	h := command.NewHandler[any](bus)
	errs, err := h.Handle(ctx, "foo", func(command.Context) error {
		return nil
	})
	if err != nil {
		t.Fatalf("handle commands: %v", err)
	}

	go func() {
		for err := range errs {
			log.Println(err)
			panic(err)
		}
	}()

	cmd := command.New("foo", struct{}{})

	t.ReportAllocs()
	t.ResetTimer()

	for i := 0; i < t.N; i++ {
		start := time.Now()
		if err := bus.Dispatch(ctx, cmd.Any(), dispatch.Sync()); err != nil {
			t.Fatalf("dispatch command: %v", err)
		}
		dur := time.Since(start)

		nanos := float64(dur) / float64(t.N)

		t.ReportMetric(nanos, "ns/op")
	}
}
