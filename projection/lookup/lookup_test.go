package lookup_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventbus"
	"github.com/modernice/goes/event/eventstore"
	"github.com/modernice/goes/helper/pick"
	"github.com/modernice/goes/projection/lookup"
)

func TestLookup(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus := eventbus.New()
	store := eventstore.WithBus(eventstore.New(), bus)

	events := []event.Event{
		event.New("foo", LookupEvent{Foo: "foo"}, event.Aggregate(uuid.New(), "foo", 1)).Any(),
		event.New("bar", LookupEvent{Foo: "bar"}, event.Aggregate(uuid.New(), "foo", 1)).Any(),
		event.New("baz", LookupEvent{Foo: "baz"}, event.Aggregate(uuid.New(), "foo", 1)).Any(),
	}

	if err := store.Insert(ctx, events...); err != nil {
		t.Fatalf("insert events: %v", err)
	}

	l := lookup.New(store, bus, []string{"foo", "bar", "baz"})
	errs, err := l.Run(ctx)
	if err != nil {
		t.Fatalf("Run() failed with %q", err)
	}

	go func() {
		for err := range errs {
			panic(err)
		}
	}()

	tests := map[event.Event]string{
		events[0]: "foo",
		events[1]: "bar",
		events[2]: "baz",
	}

	for evt, want := range tests {
		got, ok := l.Lookup(ctx, "foo", "foo", pick.AggregateID(evt))
		if !ok {
			t.Fatalf("Lookup has no value for %q", "foo")
		}
		if got != want {
			t.Fatalf("Lookup(%q) should return %q; got %q", "foo", want, got)
		}

		got, err := lookup.Expect[string](ctx, l, "foo", "foo", pick.AggregateID(evt))
		if err != nil {
			t.Fatalf("lookup.Expect() failed with %q", err)
		}
		if got != want {
			t.Fatalf("lookup.Expect() should return %q; got %q", want, got)
		}

		if _, err = lookup.Expect[int](ctx, l, "foo", "foo", pick.AggregateID(evt)); err == nil {
			t.Fatalf("lookup.Expect() should fail when provided a wrong generic type")
		}
	}
}

func TestLookup_Reverse(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus := eventbus.New()
	store := eventstore.WithBus(eventstore.New(), bus)

	events := []event.Event{
		event.New("foo", LookupEvent{Foo: "foo"}, event.Aggregate(uuid.New(), "foo", 1)).Any(),
		event.New("bar", LookupEvent{Foo: "bar"}, event.Aggregate(uuid.New(), "foo", 1)).Any(),
		event.New("baz", LookupEvent{Foo: "baz"}, event.Aggregate(uuid.New(), "foo", 1)).Any(),
	}

	if err := store.Insert(ctx, events...); err != nil {
		t.Fatalf("insert events: %v", err)
	}

	l := lookup.New(store, bus, []string{"foo", "bar", "baz"})
	errs, err := l.Run(ctx)
	if err != nil {
		t.Fatalf("Run() failed with %q", err)
	}

	go func() {
		for err := range errs {
			panic(err)
		}
	}()

	tests := map[string]event.Event{
		"foo": events[0],
		"bar": events[1],
		"baz": events[2],
	}

	for val, evt := range tests {
		got, ok := l.Reverse(ctx, "foo", "foo", val)
		if !ok {
			t.Fatalf("Lookup has no key for the value %q", val)
		}

		want := pick.AggregateID(evt)

		if got != want {
			t.Fatalf("Reverse(%q) should return %q; got %q", "foo", want, got)
		}
	}
}

func TestLookup_removed(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus := eventbus.New()
	store := eventstore.New()

	l := lookup.New(store, bus, []string{"foo", "bar"})
	errs, err := l.Run(ctx)
	if err != nil {
		t.Fatalf("Run() failed with %q", err)
	}
	go func() {
		for range errs {
		}
	}()

	aggregateID := uuid.New()

	events := []event.Event{
		event.New("foo", LookupEvent{Foo: "foo"}, event.Aggregate(aggregateID, "foo", 1)).Any(),
		event.New("bar", LookupRemoveEvent{}, event.Aggregate(aggregateID, "foo", 2)).Any(),
	}

	if err := bus.Publish(ctx, events...); err != nil {
		t.Fatalf("publish events: %v", err)
	}

	value, ok := l.Lookup(ctx, "foo", "foo", aggregateID)
	if ok {
		t.Fatalf("Lookup should return false; got true")
	}

	if value != nil {
		t.Fatalf("Lookup should return nil; got %v", value)
	}
}

// LookupEvent is a type used in testing the lookup package. It provides a Foo
// field and implements the ProvideLookup method of the lookup.Provider
// interface.
type LookupEvent struct {
	Foo string
}

// ProvideLookup adds a lookup value to the provider for the LookupEvent
// instance.
func (e LookupEvent) ProvideLookup(p lookup.Provider) {
	p.Provide("foo", e.Foo)
}

// LookupRemoveEvent is a type that, when used in the context of the lookup
// package, signals the removal of a lookup value from the provider it's
// attached to. It has no fields and its sole purpose is to implement the
// ProvideLookup method of the lookup.Provider interface in a way that instructs
// it to remove a specific value.
type LookupRemoveEvent struct{}

// ProvideLookup removes the provided lookup value from the provider for the
// LookupRemoveEvent instance.
func (LookupRemoveEvent) ProvideLookup(p lookup.Provider) {
	p.Remove("foo")
}
