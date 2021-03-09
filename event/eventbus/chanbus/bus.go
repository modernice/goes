// Package chanbus provides a simple event.Bus implementation using channels.
package chanbus

import (
	"context"
	"sync"

	"github.com/modernice/goes/event"
)

type eventBus struct {
	mux    sync.Mutex
	subs   map[string][]*subscription
	queue  chan event.Event
	add    chan *subscription
	remove chan *subscription
}

type subscription struct {
	wg    sync.WaitGroup
	bus   *eventBus
	ctx   context.Context
	names []string
	out   chan event.Event
	errs  chan error
}

// New returns a Bus that communicates over channels.
func New() event.Bus {
	bus := &eventBus{
		subs:   make(map[string][]*subscription),
		queue:  make(chan event.Event),
		add:    make(chan *subscription),
		remove: make(chan *subscription),
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
	for {
		select {
		case sub := <-bus.add:
			bus.addSub(sub)
		case sub := <-bus.remove:
			sub.wg.Wait()
			bus.removeSub(sub)
		case evt := <-bus.queue:
			bus.doPublish(evt)
		}
	}
}

func (bus *eventBus) addSub(sub *subscription) {
	bus.mux.Lock()
	defer bus.mux.Unlock()
	for _, name := range sub.names {
		bus.subs[name] = append(bus.subs[name], sub)
	}
}

func (bus *eventBus) removeSub(sub *subscription) {
	bus.mux.Lock()
	defer bus.mux.Unlock()

	close(sub.out)
	close(sub.errs)

	for _, name := range sub.names {
		for i, s := range bus.subs[name] {
			if s == sub {
				bus.subs[name] = append(bus.subs[name][:i], bus.subs[name][i+1:]...)
				break
			}
		}
	}
}

func (bus *eventBus) doPublish(evt event.Event) {
	subs := bus.subscribers(evt.Name())
	for _, sub := range subs {
		sub.wg.Add(1)
		go func(sub *subscription) {
			sub.out <- evt
			sub.wg.Done()
		}(sub)
	}
}

func (bus *eventBus) subscribers(name string) []*subscription {
	bus.mux.Lock()
	defer bus.mux.Unlock()
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
	sub := bus.newSubscription(ctx, names)
	return sub.out, sub.errs, nil
}

func (sub *subscription) handleCancel() {
	<-sub.ctx.Done()
	sub.bus.remove <- sub
}

func (bus *eventBus) newSubscription(ctx context.Context, names []string) *subscription {
	sub := &subscription{
		bus:   bus,
		ctx:   ctx,
		names: names,
		out:   make(chan event.Event),
		errs:  make(chan error),
	}
	bus.add <- sub
	go sub.handleCancel()
	return sub
}
