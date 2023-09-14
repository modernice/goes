//go:build nats

package cmdbus_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/modernice/goes/backend/nats"
	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/command"
	"github.com/modernice/goes/command/cmdbus"
	"github.com/modernice/goes/command/cmdbus/dispatch"
)

func TestBus_NATS_Core(t *testing.T) {
	ereg := codec.New()
	cmdbus.RegisterEvents(ereg)
	enc := codec.New()
	codec.Register[mockPayload](enc, "foo-cmd")
	bus := nats.NewEventBus(
		ereg,
		nats.Use(nats.Core()),
		nats.URL(os.Getenv("NATS_URL")),
		nats.SubjectPrefix("cmdbus:"),
	)
	testNATSBus(t, bus, bus)
}

func TestBus_NATS_Core_SingleBusReceivesEvent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ereg := codec.New()
	cmdbus.RegisterEvents(ereg)
	enc := codec.New()
	codec.Register[mockPayload](enc, "foo-cmd")

	ebus1 := nats.NewEventBus(
		ereg,
		nats.Use(nats.Core()),
		nats.URL(os.Getenv("NATS_URL")),
		nats.SubjectPrefix("cmdbus:"),
	)
	ebus2 := nats.NewEventBus(
		ereg,
		nats.Use(nats.Core()),
		nats.URL(os.Getenv("NATS_URL")),
		nats.SubjectPrefix("cmdbus:"),
	)

	epubBus := nats.NewEventBus(
		ereg,
		nats.Use(nats.Core()),
		nats.URL(os.Getenv("NATS_URL")),
		nats.SubjectPrefix("cmdbus:"),
	)

	bus1, _, _ := newBusWith(ctx, enc, ebus1, cmdbus.ReceiveTimeout(0), cmdbus.Debug(true))
	bus2, _, _ := newBusWith(ctx, enc, ebus2, cmdbus.ReceiveTimeout(0), cmdbus.Debug(true))
	pubBus, _, _ := newBusWith(ctx, enc, epubBus, cmdbus.AssignTimeout(0), cmdbus.Debug(true))

	commands1, errs1, err := bus1.Subscribe(ctx, "foo-cmd")
	if err != nil {
		t.Fatalf("failed to subscribe to bus1: %v", err)
	}

	commands2, errs2, err := bus2.Subscribe(ctx, "foo-cmd")
	if err != nil {
		t.Fatalf("failed to subscribe to bus2: %v", err)
	}

	newCmd := func() command.Command { return command.New("foo-cmd", mockPayload{}).Any() }
	dispatchError := make(chan error)

	go func() {
		if err := pubBus.Dispatch(ctx, newCmd(), dispatch.Sync()); err != nil {
			dispatchError <- err
		}
	}()

	var count int
	timeout := time.NewTimer(500 * time.Millisecond)
	defer timeout.Stop()
	var received []command.Command
	for {
		select {
		case err := <-errs1:
			t.Fatalf("bus1: %v", err)
		case err := <-errs2:
			t.Fatalf("bus2: %v", err)
		case err := <-dispatchError:
			t.Fatalf("dispatch: %v", err)
		case cmd := <-commands1:
			count++
			received = append(received, cmd)
		case cmd := <-commands2:
			count++
			received = append(received, cmd)
		case <-timeout.C:
			if count != 1 {
				t.Fatalf("command should have been received by exactly 1 bus; received by %d\n\t%s", count, received)
			}
			return
		}
	}
}

func TestBus_NATS_JetStream(t *testing.T) {
	ereg := codec.New()
	cmdbus.RegisterEvents(ereg)
	enc := codec.New()
	codec.Register[mockPayload](enc, "foo-cmd")
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
	enc := codec.New()
	codec.Register[mockPayload](enc, "foo-cmd")
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
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	ereg := codec.New()
	cmdbus.RegisterEvents(ereg)
	enc := codec.New()
	codec.Register[mockPayload](enc, "foo-cmd")
	subBus := cmdbus.New[int](enc, subEventBus, cmdbus.AssignTimeout(0))
	pubBus := cmdbus.New[int](enc, pubEventBus, cmdbus.AssignTimeout(0))

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
