//go:build nats

package cmdbus_test

import (
	"context"
	"math/rand"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/backend/nats"
	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/command"
	"github.com/modernice/goes/command/cmdbus"
	"github.com/modernice/goes/command/cmdbus/dispatch"
)

func BenchmarkBus_NATS_Dispatch_Synchronous(t *testing.B) {
	ereg := codec.New()
	cmdbus.RegisterEvents[uuid.UUID](ereg)
	enc := codec.Gob(codec.New())
	enc.GobRegister("foo-cmd", func() any { return mockPayload{} })
	subEventBus := nats.NewEventBus(uuid.New, ereg, nats.Use(nats.JetStream[uuid.UUID]()), nats.URL(os.Getenv("JETSTREAM_URL")))
	pubEventBus := nats.NewEventBus(uuid.New, ereg, nats.Use(nats.JetStream[uuid.UUID]()), nats.URL(os.Getenv("JETSTREAM_URL")))
	subBus := cmdbus.New[uuid.UUID](uuid.New, enc, subEventBus)
	pubBus := cmdbus.New[uuid.UUID](uuid.New, enc, pubEventBus)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	h := command.NewHandler[any, uuid.UUID](subBus)
	errs, err := h.Handle(ctx, "foo-cmd", func(command.Context) error {
		return nil
	})
	if err != nil {
		t.Fatalf("handle commands: %v", err)
	}

	go func() {
		for err := range errs {
			panic(err)
		}
	}()

	cmd := command.New(uuid.New(), "foo-cmd", mockPayload{})

	t.ReportAllocs()
	t.ResetTimer()

	for i := 0; i < t.N; i++ {
		start := time.Now()
		if err := pubBus.Dispatch(ctx, cmd.Any(), dispatch.Sync()); err != nil {
			t.Fatalf("dispatch command: %v", err)
		}
		dur := time.Since(start)

		nanos := float64(dur) / float64(t.N)

		t.ReportMetric(nanos, "ns/op")
	}
}

var clientIDs = make(map[string]bool)

func randomClientID() string {
	var id string
	for id == "" {
		rnd := rand.Int()
		id = "client_" + strconv.Itoa(rnd)
		if clientIDs[id] {
			id = ""
			continue
		}
		clientIDs[id] = true
	}
	return id
}
