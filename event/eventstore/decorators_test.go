package eventstore_test

import (
	"context"
	"testing"

	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventbus/chanbus"
	"github.com/modernice/goes/event/eventstore"
	"github.com/modernice/goes/event/eventstore/memstore"
	"github.com/modernice/goes/event/test"
)

func TestWithBus(t *testing.T) {
	s := memstore.New()
	b := chanbus.New()
	swb := eventstore.WithBus(s, b)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events, errs, err := b.Subscribe(ctx, "foo")
	if err != nil {
		t.Fatalf("failed to subscribe to Events: %v", err)
	}

	evt := event.New("foo", test.FooEventData{})
	if err := swb.Insert(ctx, evt); err != nil {
		t.Fatalf("failed to insert Event: %v", err)
	}

	var walkedEvent event.Event

	if err = event.Walk(context.Background(), func(e event.Event) {
		walkedEvent = e
		cancel()
	}, events, errs); err != nil {
		t.Fatalf("Walk shouldn't fail; failed with %q", err)
	}

	if !event.Equal(walkedEvent, evt) {
		t.Errorf("received wrong Event. want=%v got=%v", evt, walkedEvent)
	}
}
