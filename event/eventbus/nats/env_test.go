// +build nats

package nats

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/modernice/goes/event/test"
	"github.com/modernice/goes/internal/env"
	"github.com/nats-io/nats.go"
)

func TestEventBus_envQueueGroupByEvent(t *testing.T) {
	bus := New(test.NewEncoder())
	if queue := bus.queueFunc("foo"); queue != "" {
		t.Fatalf("bus.queueFunc(%q) should return %q; got %q", "foo", "", queue)
	}

	defer env.Temp("NATS_QUEUE_GROUP_BY_EVENT", true)()

	bus = New(test.NewEncoder())
	if queue := bus.queueFunc("foo"); queue != "foo" {
		t.Fatalf("bus.queueFunc(%q) should return %q; got %q", "foo", "foo", queue)
	}
}

func TestEventBus_envSubjectPrefix(t *testing.T) {
	bus := New(test.NewEncoder())
	if subject := bus.subjectFunc("foo"); subject != "foo" {
		t.Fatalf("bus.queueFunc(%q) should return %q; got %q", "foo", "foo", subject)
	}

	defer env.Temp("NATS_SUBJECT_PREFIX", "  bar.  ")() // spaces should be trimmed

	bus = New(test.NewEncoder())
	if subject := bus.subjectFunc("foo"); subject != "bar.foo" {
		t.Fatalf("bus.queueFunc(%q) should return %q; got %q", "foo", "bar.foo", subject)
	}
}

func TestEventBus_envNATSURL(t *testing.T) {
	recover := env.Temp("NATS_URL", "")
	defer recover()

	bus := New(test.NewEncoder())
	if url := bus.natsURL(); url != nats.DefaultURL {
		t.Fatalf("bus.natsURL() should return %q; got %q", nats.DefaultURL, url)
	}

	want := "foo://bar:123"
	defer env.Temp("NATS_URL", want)()

	bus = New(test.NewEncoder())
	if url := bus.natsURL(); url != want {
		t.Fatalf("bus.natsURL() should return %q; got %q", want, url)
	}
}

func TestEventBus_envReceiveTimeout(t *testing.T) {
	bus := New(test.NewEncoder())
	if bus.receiveTimeout != 0 {
		t.Fatalf("bus.receiveTimeout should be %v; got %v", 0, bus.receiveTimeout)
	}

	want := 375 * time.Millisecond
	restore := env.Temp("NATS_RECEIVE_TIMEOUT", want)

	bus = New(test.NewEncoder())
	if bus.receiveTimeout != want {
		restore()
		t.Fatalf("bus.receiveTimeout should be %v; got %v", want, bus.receiveTimeout)
	}
	restore()

	defer env.Temp("NATS_RECEIVE_TIMEOUT", "invalid")()

	bus = New(test.NewEncoder())
	errs := bus.Errors(context.Background())

	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()

	select {
	case <-timer.C:
		t.Fatalf("didn't receive error after %s", 3*time.Second)
	case err := <-errs:
		_, parseError := time.ParseDuration("invalid")
		want := fmt.Errorf("init: parse environment variable %q: %w", "NATS_RECEIVE_TIMEOUT", parseError).Error()
		if err.Error() != want {
			t.Fatalf("expected error message %q; got %q", want, err.Error())
		}
		return
	}
}
