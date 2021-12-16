//go:build nats

package natsbus

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/test"
	"github.com/nats-io/nats.go"
)

func TestQueueGroupByFunc(t *testing.T) {
	enc := test.NewEncoder()

	// given 5 "foo" subscribers
	subs := make([]<-chan event.Event, 5)
	for i := range subs {
		bus := New(enc, QueueGroupByFunc(func(eventName string) string {
			return fmt.Sprintf("bar.%s", eventName)
		}))

		events, _, err := bus.Subscribe(context.Background(), "foo")
		if err != nil {
			t.Fatal(fmt.Errorf("[%d] subscribe to %q events: %w", i, "foo", err))
		}
		subs[i] = events
	}

	pubBus := New(enc, QueueGroupByFunc(func(eventName string) string {
		return fmt.Sprintf("bar.%s", eventName)
	}))

	// when a "foo" event is published
	evt := event.New("foo", test.FooEventData{})
	err := pubBus.Publish(context.Background(), evt)
	if err != nil {
		t.Fatal(fmt.Errorf("publish event %#v: %w", evt, err))
	}

	// only 1 subscriber should received the event
	receivedChan := make(chan event.Event, len(subs))
	var wg sync.WaitGroup
	wg.Add(len(subs))
	go func() {
		defer close(receivedChan)
		wg.Wait()
	}()
	for _, events := range subs {
		go func(events <-chan event.Event) {
			defer wg.Done()
			select {
			case evt := <-events:
				receivedChan <- evt
			case <-time.After(100 * time.Millisecond):
			}
		}(events)
	}
	wg.Wait()

	var received []event.Event
	for evt := range receivedChan {
		received = append(received, evt)
	}

	if len(received) != 1 {
		t.Fatal(fmt.Errorf("expected exactly 1 subscriber to receive an event; %d subscribers received it", len(received)))
	}

	if !event.Equal(received[0], evt) {
		t.Fatal(fmt.Errorf("received event doesn't match published event\npublished: %#v\n\nreceived: %#v", evt, received[0]))
	}
}

func TestQueueGroupByEvent(t *testing.T) {
	bus := New(test.NewEncoder(), QueueGroupByEvent())
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
	bus := New(test.NewEncoder(), URL(url))
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

	bus := New(test.NewEncoder())
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
	bus := New(test.NewEncoder(), Conn(conn))

	if bus.conn.get() != conn {
		t.Fatal(fmt.Errorf("expected bus.conn to be %#v; got %#v", conn, bus.conn))
	}

	if err := bus.connectOnce(context.Background()); err != nil {
		t.Fatal(fmt.Errorf("expected bus.connectOnce not to fail; got %#v", err))
	}

	if bus.conn.get() != conn {
		t.Fatal(fmt.Errorf("expected bus.conn to still be %#v; got %#v", conn, bus.conn))
	}
}

func TestSubjectFunc(t *testing.T) {
	bus := New(test.NewEncoder(), SubjectFunc(func(eventName string) string {
		return "prefix." + eventName
	}))

	events, _, err := bus.Subscribe(context.Background(), "foo")
	if err != nil {
		t.Fatal(fmt.Errorf("subscribe to %q events: %w", "foo", err))
	}

	evt := event.New("foo", test.FooEventData{A: "foo"})
	if err = bus.Publish(context.Background(), evt); err != nil {
		t.Fatal(fmt.Errorf("publish %q event: %w", "foo", err))
	}

	select {
	case <-time.After(time.Second):
		t.Fatal(fmt.Errorf("didn't receive event after 1s"))
	case received := <-events:
		if !event.Equal(received, evt) {
			t.Fatal(fmt.Errorf("expected received event to equal %#v; got %#v", evt, received))
		}
	}
}

func TestSubjectFunc_subjectFunc(t *testing.T) {
	// default subject
	bus := New(test.NewEncoder())
	if got := bus.subjectFunc("foo"); got != "foo" {
		t.Fatal(fmt.Errorf("expected bus.subjectFunc(%q) to return %q; got %q", "foo", "foo", got))
	}

	// custom subject func
	bus = New(test.NewEncoder(), SubjectFunc(func(eventName string) string {
		return fmt.Sprintf("prefix.%s", eventName)
	}))

	want := "prefix.foo"
	if got := bus.subjectFunc("foo"); got != want {
		t.Fatal(fmt.Errorf("expected bus.subjectFunc(%q) to return %q; got %q", "foo", want, got))
	}
}

func TestSubjectPrefix(t *testing.T) {
	bus := New(test.NewEncoder(), SubjectPrefix("prefix."))

	want := "prefix.foo"
	if got := bus.subjectFunc("foo"); got != want {
		t.Fatal(fmt.Errorf("expected bus.subjectFunc(%q) to return %q; got %q", "foo", want, got))
	}
}

func TestDurableFunc(t *testing.T) {
	// default durable name
	bus := New(test.NewEncoder())
	if got := bus.durableFunc("foo", "bar"); got != "" {
		t.Fatal(fmt.Errorf("expected bus.durableFunc(%q, %q) to return %q; got %q", "foo", "bar", "", got))
	}

	// custom durable func
	bus = New(test.NewEncoder(), DurableFunc(func(subject, queue string) string {
		return fmt.Sprintf("prefix.%s.%s", subject, queue)
	}))

	want := "prefix.foo.bar"
	if got := bus.durableFunc("foo", "bar"); got != want {
		t.Fatal(fmt.Errorf("expected bus.durableFunc(%q, %q) to return %q; got %q", "foo", "bar", want, got))
	}
}

func TestDurable(t *testing.T) {
	bus := New(test.NewEncoder(), Durable())
	want := "foo_bar"
	if got := bus.durableFunc("foo", "bar"); got != want {
		t.Errorf("expected bus.durableFunc(%q, %q) to return %q; got %q", "foo", "bar", want, got)
	}
}

func TestReceiveTimeout(t *testing.T) {
	// given a receive timeout of 100ms
	timeout := 100 * time.Millisecond
	enc := test.NewEncoder()
	subBus := New(enc, ReceiveTimeout(timeout))
	pubBus := New(enc)

	// given a "foo" and "bar" subscription
	events, errs, err := subBus.Subscribe(context.Background(), "foo", "bar")
	if err != nil {
		t.Fatal(fmt.Errorf("subscribe to %q events: %w", "foo", err))
	}

	// when a "foo" event is published
	foo := event.New("foo", test.FooEventData{A: "foo"})
	if err = pubBus.Publish(context.Background(), foo); err != nil {
		t.Fatal(fmt.Errorf("publish %q event: %w", "foo", err))
	}

	// when there's no receive from events for at least 100ms
	<-time.After(150 * time.Millisecond)

	// the event should be dropped
	select {
	case evt := <-events:
		t.Fatal(fmt.Errorf("didn't expected to receive from events; got %#v", evt))
	default:
	}

	// and we should receive an error indicating that an event was dropped
	select {
	case <-time.After(100 * time.Millisecond):
		t.Fatal(fmt.Errorf("didn't receive from errs after 100ms"))
	case err := <-errs:
		if !errors.Is(err, ErrReceiveTimeout) {
			t.Fatalf("expected to receive %q error; got %q", ErrReceiveTimeout, err)
		}
	}

	// when another "bar" event is published
	bar := event.New("bar", test.BarEventData{A: "bar"})
	if err = pubBus.Publish(context.Background(), bar); err != nil {
		t.Fatal(fmt.Errorf("publish %q event: %w", "bar", err))
	}

	// the "bar" event should be received
	select {
	case <-time.After(3 * time.Second):
		t.Fatal(fmt.Errorf("didn't receive from events after 3s"))
	case evt := <-events:
		if !event.Equal(evt, bar) {
			t.Fatal(fmt.Errorf("received wrong event\nexpected: %#v\n\ngot: %#v", bar, evt))
		}
	}
}
