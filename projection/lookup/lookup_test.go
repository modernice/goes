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

type LookupEvent struct {
	Foo string
}

func (e LookupEvent) ProvideLookup(p lookup.Provider) {
	p.Provide("foo", e.Foo)
}
