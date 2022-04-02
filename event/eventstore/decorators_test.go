package eventstore_test

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventbus"
	"github.com/modernice/goes/event/eventstore"
	"github.com/modernice/goes/event/test"
	"github.com/modernice/goes/helper/streams"
)

func TestWithBus(t *testing.T) {
	s := eventstore.New()
	b := eventbus.New()
	swb := eventstore.WithBus(s, b)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events, errs, err := b.Subscribe(ctx, "foo")
	if err != nil {
		t.Fatalf("failed to subscribe to events: %v", err)
	}

	evt := event.New("foo", test.FooEventData{})
	if err := swb.Insert(ctx, evt.Any()); err != nil {
		t.Fatalf("failed to insert event: %v", err)
	}

	var walkedEvent event.Event

	if err = streams.Walk(context.Background(), func(e event.Event) error {
		walkedEvent = e
		cancel()
		return nil
	}, events, errs); err != nil {
		t.Fatalf("Walk shouldn't fail; failed with %q", err)
	}

	if !event.Equal(walkedEvent, evt.Any().Event()) {
		t.Errorf("received wrong event. want=%v got=%v\n\n%s", evt, walkedEvent, cmp.Diff(walkedEvent, evt.Any().Event()))
	}
}
