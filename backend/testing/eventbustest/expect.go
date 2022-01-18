package eventbustest

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modernice/goes/event"
)

type Expectations struct {
	ctx   context.Context
	once  sync.Once
	wg    sync.WaitGroup
	count int64
	errs  chan error
	ok    chan struct{}
}

// A Subscription wraps the event and and error channels from an event subscription
type Subscription struct {
	events <-chan event.EventOf[any]
	errs   <-chan error
}

// Sub wraps the given event and error channels in a Subscription.
func Sub(events <-chan event.EventOf[any], errs <-chan error) Subscription {
	return Subscription{events, errs}
}

// MustSub does the same as Sub, but accepts an additional error argument so it
// can be used like this:
//
//	var bus event.Bus
//	sub := MustSub(bus.Subscribe(context.TODO(), "foo"))
//
// MustSub panics if error is not nil.
func MustSub(events <-chan event.EventOf[any], errs <-chan error, err error) Subscription {
	if err != nil {
		panic(err)
	}
	return Sub(events, errs)
}

// Expect returns a new Expectation for expecting subscribed events and errors.
func Expect(ctx context.Context) *Expectations {
	return &Expectations{
		ctx:  ctx,
		errs: make(chan error),
		ok:   make(chan struct{}),
	}
}

// Result returns the result of the expectations.
func (ex *Expectations) Result() error {
	select {
	case err := <-ex.errs:
		return err
	case <-ex.ok:
		return nil
	}
}

// Apply applies the result onto the provided test. If the result is an error,
// t.Fatal(err) is called.
func (ex *Expectations) Apply(t *testing.T) {
	if err := ex.Result(); err != nil {
		t.Fatal(err)
	}
}

// Nothing expectes that nothing happens to the event and error channel of a
// subscription within the given duration.
func (ex *Expectations) Nothing(sub Subscription, timeout time.Duration) {
	count := ex.init()
	go func() {
		defer ex.wg.Done()
		start := time.Now()
		select {
		case <-ex.ctx.Done():
		case <-time.After(timeout):
		case evt, ok := <-sub.events:
			if !ok {
				ex.err("nothing", count, fmt.Errorf("event channel is closed [duration=%v]", time.Since(start)))
				return
			}
			ex.err("nothing", count, fmt.Errorf("received event: %v [duration=%v]", evt.Name(), time.Since(start)))
		case err, ok := <-sub.errs:
			if !ok {
				ex.err("nothing", count, fmt.Errorf("error channel is closed [duration=%v]", time.Since(start)))
				return
			}
			ex.err("nothing", count, fmt.Errorf("received error: %v [duration=%v]", err, time.Since(start)))
		}
	}()
}

// Event expects any of the given events to be received within the given duration.
func (ex *Expectations) Event(sub Subscription, timeout time.Duration, names ...string) {
	count := ex.init()
	go func() {
		defer ex.wg.Done()
		start := time.Now()
		for {
			select {
			case <-ex.ctx.Done():
				return
			case <-time.After(timeout):
				ex.err("event", count, fmt.Errorf("event not received [event=anyOf(%v), duration=%v]", names, time.Since(start)))
				return
			case evt, ok := <-sub.events:
				if !ok {
					ex.err("event", count, fmt.Errorf("event channel is closed [duration=%v]", time.Since(start)))
					return
				}
				for _, name := range names {
					if evt.Name() == name {
						return
					}
				}
			case err, ok := <-sub.errs:
				if !ok {
					ex.err("event", count, fmt.Errorf("error channel is closed [duration=%v]", time.Since(start)))
					return
				}
				ex.err("event", count, fmt.Errorf("received error: %v [duration=%v]", err, time.Since(start)))
				return
			}
		}
	}()
}

// Events expects all of the given events to be received within the given duration.
func (ex *Expectations) Events(sub Subscription, timeout time.Duration, names ...string) {
	count := ex.init()
	go func() {
		defer ex.wg.Done()
		start := time.Now()
		var received []string
		for {
			select {
			case <-ex.ctx.Done():
				return
			case <-time.After(timeout):
				ex.err("event", count, fmt.Errorf("events not received [events=%v, received=%v, duration=%v]", names, received, time.Since(start)))
				return
			case evt, ok := <-sub.events:
				if !ok {
					ex.err("event", count, fmt.Errorf("event channel is closed [duration=%v]", time.Since(start)))
					return
				}
				for _, name := range names {
					if evt.Name() == name {
						received = append(received, name)
						break
					}
				}

				done := true
				for _, name := range names {
					var found bool
					for _, evt := range received {
						if name == evt {
							found = true
							break
						}
					}
					if !found {
						done = false
						break
					}
				}

				if done {
					return
				}
			case err, ok := <-sub.errs:
				if !ok {
					ex.err("event", count, fmt.Errorf("error channel is closed [duration=%v]", time.Since(start)))
					return
				}
				ex.err("event", count, fmt.Errorf("received error: %v [duration=%v]", err, time.Since(start)))
				return
			}
		}
	}()
}

// Closed expects the event and error channels of a subscription to be closed
// within the given duration.
func (ex *Expectations) Closed(sub Subscription, timeout time.Duration) {
	count := ex.init()
	go func() {
		defer ex.wg.Done()
		start := time.Now()
		select {
		case <-ex.ctx.Done():
			return
		case <-time.After(timeout):
			ex.err("closed", count, fmt.Errorf("channels not closed [duration=%v]", time.Since(start)))
			return
		case _, ok := <-sub.events:
			if ok {
				ex.err("closed", count, fmt.Errorf("event channel not closed [duration=%v]", time.Since(start)))
				return
			}
		case _, ok := <-sub.errs:
			if ok {
				ex.err("closed", count, fmt.Errorf("error channel not closed [duration=%v]", time.Since(start)))
				return
			}
		}
	}()
}

func (ex *Expectations) init() int64 {
	ex.wg.Add(1)
	ex.once.Do(func() { go ex.work() })
	return atomic.AddInt64(&ex.count, 1)
}

func (ex *Expectations) work() {
	ex.wg.Wait()
	close(ex.ok)
}

func (ex *Expectations) err(fn string, count int64, err error) {
	err = fmt.Errorf("[expectations.%s:%d] %v", fn, count, err)
	log.Println(err)
	select {
	case <-ex.ctx.Done():
	case ex.errs <- err:
	}
}
