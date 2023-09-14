//go:build nats

package nats

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/test"
	"github.com/modernice/goes/helper/streams"
	"github.com/modernice/goes/internal/slice"
	"github.com/nats-io/nats.go"
)

func TestLoadBalancer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	enc := test.NewEncoder()

	// given 5 event buses with the same queue group
	buses := make([]*EventBus, 5)
	for i := range buses {
		buses[i] = NewEventBus(enc, LoadBalancer("queue"))
	}

	// that are subscribed to "foo" events
	var subErrors []<-chan error
	subEvents := slice.Map(buses, func(bus *EventBus) <-chan event.Event {
		events, errs, err := bus.Subscribe(ctx, "foo")
		if err != nil {
			t.Fatalf("subscribe to %q events: %v", "foo", err)
		}
		subErrors = append(subErrors, errs)
		return events
	})
	errs := streams.FanInAll(subErrors...)
	events := streams.FanInAll(subEvents...)

	// and a publisher bus
	pubBus := NewEventBus(enc)

	// when we publish an event via the publisher bus
	evt := event.New("foo", test.FooEventData{A: "foo"})
	if err := pubBus.Publish(ctx, evt.Any()); err != nil {
		t.Fatalf("publish event: %v", err)
	}

	// it should be received by a single subscribed bus
	var count int
	timeout := time.NewTimer(200 * time.Millisecond)
	defer timeout.Stop()
	for {
		select {
		case err := <-errs:
			t.Fatal(err)
		case <-events:
			count++
		case <-timeout.C:
			if count != 1 {
				t.Fatalf("event should have been received by 1 bus; received by %d", count)
			}
			return
		}
	}
}

func TestURL(t *testing.T) {
	url := "foo://bar:123"
	bus := NewEventBus(test.NewEncoder(), URL(url))
	if bus.natsURL() != url {
		t.Fatal(fmt.Errorf("expected bus.natsURL to return %q; got %q", url, bus.natsURL()))
	}
}

func TestEventBus_natsURL(t *testing.T) {
	envURL := "foo://bar:123"
	org := os.Getenv("NATS_URL")
	if err := os.Setenv("NATS_URL", envURL); err != nil {
		t.Fatal(fmt.Errorf("set env %q=%q: %w", "NATS_URL", envURL, err))
	}
	defer func() {
		if err := os.Setenv("NATS_URL", org); err != nil {
			t.Fatal(fmt.Errorf("set env %q=%q: %w", "NATS_URL", org, err))
		}
	}()

	bus := NewEventBus(test.NewEncoder())
	if bus.natsURL() != envURL {
		t.Fatal(fmt.Errorf("expected bus.natsURL to return %q; got %q", envURL, bus.natsURL()))
	}

	bus.url = "bar://foo:321"
	if bus.natsURL() != bus.url {
		t.Fatal(fmt.Errorf("expected bus.natsURL to return %q; got %q", bus.url, bus.natsURL()))
	}
}

func TestConnection(t *testing.T) {
	conn := &nats.Conn{}
	bus := NewEventBus(test.NewEncoder(), Conn(conn))

	if bus.conn != conn {
		t.Fatal(fmt.Errorf("expected bus.conn to be %v; got %v", conn, bus.conn))
	}

	if err := bus.Connect(context.Background()); err != nil {
		t.Fatal(fmt.Errorf("expected bus.connectOnce not to fail; got %v", err))
	}

	if bus.conn != conn {
		t.Fatal(fmt.Errorf("expected bus.conn to still be %v; got %v", conn, bus.conn))
	}
}

func TestSubjectFunc(t *testing.T) {
	bus := NewEventBus(test.NewEncoder(), SubjectFunc(func(eventName string) string {
		return "prefix." + eventName
	}))

	events, _, err := bus.Subscribe(context.Background(), "foo")
	if err != nil {
		t.Fatal(fmt.Errorf("subscribe to %q events: %w", "foo", err))
	}

	evt := event.New("foo", test.FooEventData{A: "foo"})
	if err := bus.Publish(context.Background(), evt.Any().Event()); err != nil {
		t.Fatal(fmt.Errorf("publish %q event: %w", "foo", err))
	}

	select {
	case <-time.After(time.Second):
		t.Fatal(fmt.Errorf("didn't receive event after 1s"))
	case received := <-events:
		if !event.Equal(received, evt.Any().Event()) {
			t.Fatal(fmt.Errorf("expected received event to equal %v; got %v", evt, received))
		}
	}
}

func TestSubjectFunc_subjectFunc(t *testing.T) {
	// default subject
	bus := NewEventBus(test.NewEncoder())
	if got := bus.subjectFunc("foo"); got != "foo" {
		t.Fatal(fmt.Errorf("expected bus.subjectFunc(%q) to return %q; got %q", "foo", "foo", got))
	}

	// custom subject func
	bus = NewEventBus(test.NewEncoder(), SubjectFunc(func(eventName string) string {
		return fmt.Sprintf("prefix.%s", eventName)
	}))

	want := "prefix_foo"
	if got := bus.subjectFunc("foo"); got != want {
		t.Fatal(fmt.Errorf("expected bus.subjectFunc(%q) to return %q; got %q", "foo", want, got))
	}
}

func TestSubjectPrefix(t *testing.T) {
	bus := NewEventBus(test.NewEncoder(), SubjectPrefix("prefix."))

	want := "prefix_foo"
	if got := bus.subjectFunc("foo"); got != want {
		t.Fatal(fmt.Errorf("expected bus.subjectFunc(%q) to return %q; got %q", "foo", want, got))
	}
}

func TestDurableFunc(t *testing.T) {
	// default durable name
	js := JetStream().(*jetStream)
	if got := js.durableFunc("foo", "bar"); got != "" {
		t.Fatal(fmt.Errorf("expected bus.durableFunc(%q, %q) to return %q; got %q", "foo", "bar", "", got))
	}

	// custom durable func
	js = JetStream(DurableFunc(func(event, queue string) string {
		return fmt.Sprintf("prefix.%s.%s", event, queue)
	})).(*jetStream)

	want := "prefix_foo_bar"
	if got := js.durableFunc("foo", "bar"); got != want {
		t.Fatal(fmt.Errorf("expected bus.durableFunc(%q, %q) to return %q; got %q", "foo", "bar", want, got))
	}
}

func TestPullTimeout(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// given a pull timeout of 100ms
	timeout := 100 * time.Millisecond
	enc := test.NewEncoder()
	subBus := NewEventBus(enc, PullTimeout(timeout))
	pubBus := NewEventBus(enc)

	// given a "foo" and "bar" subscription
	events, errs, err := subBus.Subscribe(ctx, "foo", "bar")
	if err != nil {
		t.Fatal(fmt.Errorf("subscribe to %q events: %w", "foo", err))
	}

	go func() {
		// when a "foo" event is published
		foo := event.New("foo", test.FooEventData{A: "foo bar baz"})
		if err := pubBus.Publish(ctx, foo.Any()); err != nil {
			panic(fmt.Errorf("publish %q event: %w", "foo", err))
		}
	}()
	start := time.Now()

	// when there's no receive from events for at least 100ms
	<-time.After(500 * time.Millisecond)

	// the event should be dropped
	select {
	case evt := <-events:
		log.Printf("Received after %v", time.Since(start))
		t.Fatal(fmt.Errorf("didn't expect to receive from events; got %v", evt))
	default:
	}

	// and we should receive an error indicating that an event was dropped
	select {
	case <-time.After(100 * time.Millisecond):
		t.Fatal(fmt.Errorf("didn't receive from errs after 100ms"))
	case err := <-errs:
		if !errors.Is(err, ErrPullTimeout) {
			t.Fatalf("expected to receive %q error; got %q", ErrPullTimeout, err)
		}
	}

	// when another "bar" event is published
	bar := event.New("bar", test.BarEventData{A: "bar"})
	if err := pubBus.Publish(ctx, bar.Any()); err != nil {
		t.Fatal(fmt.Errorf("publish %q event: %w", "bar", err))
	}

	// the "bar" event should be received
	select {
	case <-time.After(3 * time.Second):
		t.Fatal(fmt.Errorf("didn't receive from events after 3s"))
	case evt := <-events:
		if !event.Equal(evt, bar.Any().Event()) {
			t.Fatal(fmt.Errorf("received wrong event\nexpected: %v\n\ngot: %v", bar, evt))
		}
	}
}
