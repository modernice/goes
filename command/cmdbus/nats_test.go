//go:build nats

package cmdbus_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/modernice/goes/backend/nats"
	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/command"
	"github.com/modernice/goes/command/cmdbus"
	"github.com/modernice/goes/command/cmdbus/dispatch"
)

func TestBus_NATS_Core(t *testing.T) {
	ereg := codec.New()
	cmdbus.RegisterEvents(ereg)
	enc := codec.Gob(codec.New())
	enc.GobRegister("foo-cmd", func() any { return mockPayload{} })
	bus := nats.NewEventBus(
		ereg,
		nats.Use(nats.Core()),
		nats.URL(os.Getenv("NATS_URL")),
	)
	testNATSBus(t, bus)
}

func TestBus_NATS_JetStream(t *testing.T) {
	ereg := codec.New()
	cmdbus.RegisterEvents(ereg)
	enc := codec.Gob(codec.New())
	enc.GobRegister("foo-cmd", func() any { return mockPayload{} })
	bus := nats.NewEventBus(
		ereg,
		nats.Use(nats.JetStream()),
		nats.URL(os.Getenv("JETSTREAM_URL")),
	)
	testNATSBus(t, bus)
}

func testNATSBus(t *testing.T, ebus *nats.EventBus) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ereg := codec.New()
	cmdbus.RegisterEvents(ereg)
	enc := codec.Gob(codec.New())
	enc.GobRegister("foo-cmd", func() any { return mockPayload{} })
	bus := cmdbus.New(enc.Registry, ebus, cmdbus.AssignTimeout(0))

	commands, errs, err := bus.Subscribe(ctx, "foo-cmd")
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	handleErrors := make(chan error)
	var handled []command.Context
	go func() {
		for {
			select {
			case ctx, ok := <-commands:
				if !ok {
					return
				}
				handled = append(handled, ctx)
				if err := ctx.Finish(ctx); err != nil {
					handleErrors <- fmt.Errorf("mark done: %w", err)
				}
			case err, ok := <-errs:
				if !ok {
					errs = nil
					break
				}
				handleErrors <- err
			}
		}
	}()

	dispatchErrors := make(chan error)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 10; i++ {
			cmd := command.New("foo-cmd", mockPayload{})
			if err := bus.Dispatch(ctx, cmd.Any(), dispatch.Sync()); err != nil {
				dispatchErrors <- fmt.Errorf("[%d] Dispatch shouldn't fail; failed with %q", i, err)
				return
			}
		}
	}()

L:
	for {
		select {
		case err := <-handleErrors:
			t.Fatal(err)
		case err := <-dispatchErrors:
			t.Fatal(err)
		case <-done:
			break L
		}
	}

	if len(handled) != 10 {
		t.Fatalf("Command should have been handled %d times; got %d", 10, len(handled))
	}
}
