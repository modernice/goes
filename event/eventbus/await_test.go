package eventbus_test

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventbus"
	"github.com/modernice/goes/event/test"
)

func TestAwaiter_Once(t *testing.T) {
	bus := eventbus.New()
	aw := eventbus.NewAwaiter(bus)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	str, errs, err := aw.Once(ctx, "foo")
	if err != nil {
		t.Fatalf("Once() failed with %q", err)
	}

	data := test.FooEventData{A: "bar"}
	go func() {
		for i := 0; i < 3; i++ {
			evt := event.New("foo", data)
			if err := bus.Publish(ctx, evt); err != nil {
				panic(err)
			}
		}
	}()

	var count int
L:
	for {
		select {
		case err := <-errs:
			t.Fatal(err)
		case evt, ok := <-str:
			if !ok {
				break L
			}

			if !cmp.Equal(evt.Data(), data) {
				t.Fatalf("wrong event data.\n\n%s", cmp.Diff(data, evt.Data()))
			}

			count++
		}
	}

	if count != 1 {
		t.Fatalf("Event should be received exactly once")
	}
}
