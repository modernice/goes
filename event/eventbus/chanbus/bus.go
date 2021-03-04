// Package chanbus provides a simple event.Bus implementation using channels.
package chanbus

import (
	"context"
	"sync"

	"github.com/modernice/goes/event"
)

type eventBus struct {
	subsMux sync.RWMutex
	subs    map[string][]*subscription
	queue   chan event.Event
}

type subscription struct {
	mux     sync.Mutex
	wg      sync.WaitGroup
	bus     *eventBus
	ctx     context.Context
	names   []string
	out     chan event.Event
	removed chan struct{}
}

// New returns a Bus that communicates over channels.
func New() event.Bus {
	bus := &eventBus{
		subs:  make(map[string][]*subscription),
		queue: make(chan event.Event),
	}
	go bus.work()
	return bus
}

// Publish sends events to the channels that have been returned by previous
// calls to bus.Subscribe() where the subscribed Event name matches the
// evt.Name() for an Event in events. If ctx is canceled before every Event has
// been queued, ctx.Err() is returned.
func (bus *eventBus) Publish(ctx context.Context, events ...event.Event) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	for _, evt := range events {
		if err := bus.publish(ctx, evt); err != nil {
			return err
		}
	}

	return nil
}

func (bus *eventBus) publish(ctx context.Context, evt event.Event) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case bus.queue <- evt:
		return nil
	}
}

func (bus *eventBus) work() {
	for evt := range bus.queue {
		subs := bus.subscriptions(evt.Name())
		for _, sub := range subs {
			sub.publish(evt)
		}
	}
}

func (bus *eventBus) subscriptions(name string) []*subscription {
	bus.subsMux.RLock()
	defer bus.subsMux.RUnlock()
	return bus.subs[name]
}

// Subscribe returns a channel of Events. For every published Event evt where
// evt.Name() is one of names, that Event will be received from the returned
// Events channel. When ctx is canceled, events won't accept any new Events and
// will be closed.
func (bus *eventBus) Subscribe(ctx context.Context, names ...string) (<-chan event.Event, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	return bus.newSubscription(ctx, names).out, nil
}

func (bus *eventBus) add(sub *subscription) {
	bus.subsMux.Lock()
	defer bus.subsMux.Unlock()
	for _, name := range sub.names {
		bus.subs[name] = append(bus.subs[name], sub)
	}
}

func (bus *eventBus) remove(sub *subscription) {
	bus.subsMux.Lock()
	defer bus.subsMux.Unlock()
	for _, name := range sub.names {
	L:
		for i, s := range bus.subs[name] {
			if s == sub {
				bus.subs[name] = append(bus.subs[name][:i], bus.subs[name][i+1:]...)

				go func() {
					close(s.removed)
					sub.wg.Wait()
					close(sub.out)
				}()

				break L
			}
		}
	}
}

func (sub *subscription) publish(evt event.Event) {
	select {
	case <-sub.removed:
		return
	default:
		sub.wg.Add(1)
		go func() {
			defer sub.wg.Done()
			sub.out <- evt
		}()
	}
}

func (sub *subscription) handleCancel() {
	<-sub.ctx.Done()
	sub.bus.remove(sub)
}

func (bus *eventBus) newSubscription(ctx context.Context, names []string) *subscription {
	sub := &subscription{
		bus:     bus,
		ctx:     ctx,
		names:   names,
		out:     make(chan event.Event),
		removed: make(chan struct{}),
	}
	bus.add(sub)
	go sub.handleCancel()
	return sub
}
