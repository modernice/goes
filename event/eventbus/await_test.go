package eventbus_test

import (
	"context"
	"testing"
	"time"

	"github.com/modernice/goes/backend/testing/eventbustest"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventbus"
	"github.com/modernice/goes/event/test"
)

func TestAwaiter_Once(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus := eventbus.New()
	aw := eventbus.NewAwaiter[any](bus)

	// Await a "foo" event once
	sub := eventbustest.MustSub(aw.Once(ctx, "foo"))

	// When "foo" event is published 3 times, it should be received exactly once
	ex := eventbustest.Expect(ctx)
	ex.Event(sub, 50*time.Millisecond, "foo")

	for i := 0; i < 3; i++ {
		evt := event.New[any]("foo", test.FooEventData{})
		if err := bus.Publish(ctx, evt); err != nil {
			t.Fatalf("publish event: %v [event=%v, iter=%d]", err, evt.Name(), i)
		}
	}

	ex.Apply(t)

	// And then the channels should be closed.
	ex = eventbustest.Expect(ctx)
	ex.Closed(sub, 50*time.Millisecond)
	ex.Apply(t)
}
