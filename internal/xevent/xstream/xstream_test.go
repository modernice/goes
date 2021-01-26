package xstream_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/stream"
	"github.com/modernice/goes/event/test"
	"github.com/modernice/goes/internal/xevent/xstream"
)

func TestDelayed(t *testing.T) {
	events := []event.Event{
		event.New("foo", test.FooEventData{}),
		event.New("foo", test.FooEventData{}),
		event.New("foo", test.FooEventData{}),
	}

	base := stream.InMemory(events...)
	delay := 50 * time.Millisecond
	cur := xstream.Delayed(base, delay)

	var nextMux sync.Mutex
	next := func() <-chan event.Event {
		ch := make(chan event.Event, 1)
		go func() {
			nextMux.Lock()
			defer nextMux.Unlock()
			if !cur.Next(context.Background()) {
				return
			}
			ch <- cur.Event()
		}()
		return ch
	}

	var received []event.Event

	for {
		if len(received) == len(events) {
			break
		}

		func() {
			ndelay := time.Duration(float64(delay) * 0.8)
			ntimer := time.NewTimer(ndelay)
			defer ntimer.Stop()

			mdelay := time.Duration(float64(delay) * 1.2)
			mtimer := time.NewTimer(mdelay)
			defer mtimer.Stop()

			start := time.Now()

			n := next()

			select {
			case <-n:
				dur := time.Now().Sub(start)
				t.Fatalf("received event after %s; expected a delay of at least %s (%s * 80%%)", dur, ndelay, delay)
			case <-ntimer.C:
			}

			<-mtimer.C

			timer := time.NewTimer(100 * time.Millisecond)
			defer timer.Stop()

			select {
			case <-timer.C:
				t.Fatalf("event not received after %s", 100*time.Millisecond)
			case evt := <-n:
				received = append(received, evt)
			}
		}()
	}

	if err := cur.Close(context.Background()); err != nil {
		t.Errorf("cur.Close should not fail: %v", err)
	}

	test.AssertEqualEvents(t, events, received)
}
