// +build nats

package cmdbus_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/modernice/goes/command"
	"github.com/modernice/goes/command/cmdbus"
	"github.com/modernice/goes/command/cmdbus/dispatch"
	"github.com/modernice/goes/command/encoding"
	eventenc "github.com/modernice/goes/event/encoding"
	"github.com/modernice/goes/event/eventbus/natsbus"
	"github.com/nats-io/stan.go"
)

func TestBus_NATS(t *testing.T) {
	ereg := eventenc.NewGobEncoder()
	cmdbus.RegisterEvents(ereg)
	enc := encoding.NewGobEncoder()
	enc.Register("foo-cmd", func() command.Payload { return mockPayload{} })
	subEventBus := natsbus.New(
		ereg,
		natsbus.Use(natsbus.Streaming(
			"test-cluster",
			"test_client_sub",
			stan.NatsURL(os.Getenv("STAN_URL")),
		)),
	)
	pubEventBus := natsbus.New(
		ereg,
		natsbus.Use(natsbus.Streaming(
			"test-cluster",
			"test_client_pub",
			stan.NatsURL(os.Getenv("STAN_URL")),
		)),
	)
	subBus := cmdbus.New(enc, subEventBus)
	pubBus := cmdbus.New(enc, pubEventBus)

	commands, errs, err := subBus.Subscribe(context.Background(), "foo-cmd")
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
				if err := ctx.Finish(context.Background()); err != nil {
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
			if err := pubBus.Dispatch(context.Background(), cmd, dispatch.Synchronous()); err != nil {
				dispatchErrors <- fmt.Errorf("[%d] Dispatch shouldn't fail; failed with %q", i, err)
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
