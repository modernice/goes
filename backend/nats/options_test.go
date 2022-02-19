//go:build nats

package nats

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/test"
	"github.com/nats-io/nats.go"
)

func TestQueueGroupByFunc(t *testing.T) {
	enc := test.NewEncoder()

	// given 5 "foo" subscribers
	subs := make([]<-chan event.Of[any, uuid.UUID], 5)
	for i := range subs {
		bus := NewEventBus(uuid.New, enc, QueueGroupByFunc(func(eventName string) string {
			return fmt.Sprintf("bar.%s", eventName)
		}))

		events, errs, err := bus.Subscribe(context.Background(), "foo")
		if err != nil {
			t.Fatal(fmt.Errorf("[%d] subscribe to %q events: %w", i, "foo", err))
		}
		subs[i] = events

		go func() {
			for err := range errs {
				panic(err)
			}
		}()
	}

	pubBus := NewEventBus(uuid.New, enc)

	// when a "foo" event is published
	evt := event.New(uuid.New(), "foo", test.FooEventData{})
	err := pubBus.Publish(context.Background(), evt.Any())
	if err != nil {
		t.Fatal(fmt.Errorf("publish event %v: %w", evt, err))
	}

	// only 1 subscriber should received the event
	receivedChan := make(chan event.Of[any, uuid.UUID], len(subs))
	var wg sync.WaitGroup
	wg.Add(len(subs))
	go func() {
		defer close(receivedChan)
		wg.Wait()
	}()
	for _, events := range subs {
		go func(events <-chan event.Of[any, uuid.UUID]) {
			defer wg.Done()
			select {
			case evt := <-events:
				receivedChan <- evt
			case <-time.After(100 * time.Millisecond):
			}
		}(events)
	}
	wg.Wait()

	var received []event.Of[any, uuid.UUID]
	for evt := range receivedChan {
		received = append(received, evt)
	}

	if len(received) != 1 {
		t.Fatal(fmt.Errorf("expected exactly 1 subscriber to receive an event; %d subscribers received it", len(received)))
	}

	if !event.Equal(received[0], evt.Any().Event()) {
		t.Fatal(fmt.Errorf("received event doesn't match published event\npublished: %v\n\nreceived: %v", evt, received[0]))
	}
}

func TestQueueGroupByEvent(t *testing.T) {
	bus := NewEventBus(uuid.New, test.NewEncoder(), QueueGroupByEvent())
	names := []string{"foo", "bar", "baz"}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			if queue := bus.queueFunc(name); queue != name {
				t.Fatal(fmt.Errorf("expected queueFunc to return %q; got %q", name, queue))
			}
		})
	}
}

func TestURL(t *testing.T) {
	url := "foo://bar:123"
	bus := NewEventBus(uuid.New, test.NewEncoder(), URL(url))
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

	bus := NewEventBus(uuid.New, test.NewEncoder())
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
	bus := NewEventBus(uuid.New, test.NewEncoder(), Conn(conn))

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
	bus := NewEventBus(uuid.New, test.NewEncoder(), SubjectFunc(func(eventName string) string {
		return "prefix." + eventName
	}))

	events, _, err := bus.Subscribe(context.Background(), "foo")
	if err != nil {
		t.Fatal(fmt.Errorf("subscribe to %q events: %w", "foo", err))
	}

	evt := event.New(uuid.New(), "foo", test.FooEventData{A: "foo"})
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
	bus := NewEventBus(uuid.New, test.NewEncoder())
	if got := bus.subjectFunc("foo"); got != "foo" {
		t.Fatal(fmt.Errorf("expected bus.subjectFunc(%q) to return %q; got %q", "foo", "foo", got))
	}

	// custom subject func
	bus = NewEventBus(uuid.New, test.NewEncoder(), SubjectFunc(func(eventName string) string {
		return fmt.Sprintf("prefix.%s", eventName)
	}))

	want := "prefix_foo"
	if got := bus.subjectFunc("foo"); got != want {
		t.Fatal(fmt.Errorf("expected bus.subjectFunc(%q) to return %q; got %q", "foo", want, got))
	}
}

func TestSubjectPrefix(t *testing.T) {
	bus := NewEventBus(uuid.New, test.NewEncoder(), SubjectPrefix("prefix."))

	want := "prefix_foo"
	if got := bus.subjectFunc("foo"); got != want {
		t.Fatal(fmt.Errorf("expected bus.subjectFunc(%q) to return %q; got %q", "foo", want, got))
	}
}

func TestDurableFunc(t *testing.T) {
	// default durable name
	bus := NewEventBus(uuid.New, test.NewEncoder())
	if got := bus.durableFunc("foo", "bar"); got != "" {
		t.Fatal(fmt.Errorf("expected bus.durableFunc(%q, %q) to return %q; got %q", "foo", "bar", "", got))
	}

	// custom durable func
	bus = NewEventBus(uuid.New, test.NewEncoder(), DurableFunc(func(subject, queue string) string {
		return fmt.Sprintf("prefix.%s.%s", subject, queue)
	}))

	want := "prefix.foo.bar"
	if got := bus.durableFunc("foo", "bar"); got != want {
		t.Fatal(fmt.Errorf("expected bus.durableFunc(%q, %q) to return %q; got %q", "foo", "bar", want, got))
	}
}

func TestDurable(t *testing.T) {
	bus := NewEventBus(uuid.New, test.NewEncoder(), Durable("foo-dur"))
	want := "foo-dur"
	if got := bus.durableFunc("foo", "bar"); got != want {
		t.Errorf("expected bus.durableFunc(%q, %q) to return %q; got %q", "foo", "bar", want, got)
	}
}

func TestPullTimeout(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// given a pull timeout of 100ms
	timeout := 100 * time.Millisecond
	enc := test.NewEncoder()
	subBus := NewEventBus(uuid.New, enc, PullTimeout(timeout))
	pubBus := NewEventBus(uuid.New, enc)

	// given a "foo" and "bar" subscription
	events, errs, err := subBus.Subscribe(ctx, "foo", "bar")
	if err != nil {
		t.Fatal(fmt.Errorf("subscribe to %q events: %w", "foo", err))
	}

	go func() {
		// when a "foo" event is published
		foo := event.New(uuid.New(), "foo", test.FooEventData{A: "foo bar baz"})
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
	bar := event.New(uuid.New(), "bar", test.BarEventData{A: "bar"})
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
