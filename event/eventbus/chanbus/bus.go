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
	errs    chan error
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

// Publish sends the given Events to subscribers of those Events. If ctx is
// canceled before every Event has been enqueued, Publish returns ctx.Err().
func (bus *eventBus) Publish(ctx context.Context, events ...event.Event) error {
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
			go sub.publish(evt)
		}
	}
}

func (bus *eventBus) subscriptions(name string) []*subscription {
	bus.subsMux.Lock()
	defer bus.subsMux.Unlock()
	return bus.subs[name]
}

// Subscribe returns a channel of Events and a channel of asynchronous errors.
// Only Events whose name is one of the provided names will be received from the
// returned Event channel.
//
// The returned error channel will never receive any errors.
//
// When ctx is canceled, the both the Event and error channel are closed.
func (bus *eventBus) Subscribe(ctx context.Context, names ...string) (<-chan event.Event, <-chan error, error) {
	select {
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	default:
	}

	sub := bus.newSubscription(ctx, names)
	return sub.out, sub.errs, nil
}

func (bus *eventBus) add(sub *subscription) {
	bus.subsMux.Lock()
	defer bus.subsMux.Unlock()
	for _, name := range sub.names {
		bus.subs[name] = append(bus.subs[name], sub)
	}
}

func (bus *eventBus) remove(sub *subscription) {
	sub.mux.Lock()
	bus.subsMux.Lock()
	defer func() {
		sub.mux.Unlock()
		bus.subsMux.Unlock()
	}()
	for _, name := range sub.names {
	L:
		for i, s := range bus.subs[name] {
			if s == sub {
				newSubs := make([]*subscription, len(bus.subs[name])-1)
				newSubs = append(newSubs, bus.subs[name][:i]...)
				newSubs = append(newSubs, bus.subs[name][i+1:]...)
				bus.subs[name] = newSubs
				close(s.removed)
				close(s.out)

				break L
			}
		}
	}
}

func (sub *subscription) publish(evt event.Event) {
	sub.mux.Lock()
	go func() {
		defer sub.mux.Unlock()
		select {
		case <-sub.removed:
			return
		default:
			sub.out <- evt
		}
	}()
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
		errs:    make(chan error),
		removed: make(chan struct{}),
	}
	bus.add(sub)
	go sub.handleCancel()
	return sub
}
