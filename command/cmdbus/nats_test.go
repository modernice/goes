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
		nats.SubjectPrefix("cmdbus:"),
	)
	testNATSBus(t, bus, bus)
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
		nats.SubjectPrefix("cmdbus_js:"),
	)
	defer cleanup(bus)
	testNATSBus(t, bus, bus)
}

func TestBus_NATS_JetStream_Durable(t *testing.T) {
	ereg := codec.New()
	cmdbus.RegisterEvents(ereg)
	enc := codec.Gob(codec.New())
	enc.GobRegister("foo-cmd", func() any { return mockPayload{} })
	subBus := nats.NewEventBus(
		ereg,
		nats.Use(nats.JetStream(nats.Durable("cmdbus_sub"))),
		nats.URL(os.Getenv("JETSTREAM_URL")),
		nats.SubjectPrefix("cmdbus_durable:"),
	)
	pubBus := nats.NewEventBus(
		ereg,
		nats.Use(nats.JetStream(nats.Durable("cmdbus_pub"))),
		nats.URL(os.Getenv("JETSTREAM_URL")),
		nats.SubjectPrefix("cmdbus_durable:"),
	)

	defer cleanup(subBus)
	defer cleanup(pubBus)

	testNATSBus(t, subBus, pubBus)
}

func testNATSBus(t *testing.T, subEventBus, pubEventBus *nats.EventBus) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ereg := codec.New()
	cmdbus.RegisterEvents(ereg)
	enc := codec.Gob(codec.New())
	enc.GobRegister("foo-cmd", func() any { return mockPayload{} })
	subBus := cmdbus.New(enc.Registry, subEventBus, cmdbus.AssignTimeout(0))
	pubBus := cmdbus.New(enc.Registry, pubEventBus, cmdbus.AssignTimeout(0))

	commands, errs, err := subBus.Subscribe(ctx, "foo-cmd")
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
			if err := pubBus.Dispatch(ctx, cmd.Any(), dispatch.Sync()); err != nil {
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

func cleanup(bus *nats.EventBus) {
	js, err := bus.Connection().JetStream()
	if err != nil {
		return
	}

	streams := js.StreamNames()
	for stream := range streams {
		consumers := js.ConsumerNames(stream)
		for cons := range consumers {
			js.DeleteConsumer(stream, cons)
		}
		js.DeleteStream(stream)
	}
}
