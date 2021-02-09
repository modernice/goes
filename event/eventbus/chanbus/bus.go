// Package chanbus provides a simple event.Bus implementation using channels.
package chanbus

import (
	"context"
	"sync"

	"github.com/modernice/goes/event"
)

type eventBus struct {
	mux   sync.RWMutex
	subs  map[string]map[subscriber]bool
	queue chan event.Event
}

type subscriber struct {
	*sync.RWMutex

	ctx    context.Context
	events chan event.Event
	done   chan struct{}
}

// New returns a Bus that communicates over channels.
func New() event.Bus {
	bus := &eventBus{
		subs:  make(map[string]map[subscriber]bool),
		queue: make(chan event.Event),
	}
	go bus.run()
	return bus
}

// Publish sends events to the channels that have been returned by previous
// calls to bus.Subscribe() where the subscribed Event name matches the
// evt.Name() for an Event in events. If ctx is canceled before every Event has
// been queued, ctx.Err() is returned.
func (bus *eventBus) Publish(ctx context.Context, events ...event.Event) error {
	for _, evt := range events {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case bus.queue <- evt:
		}
	}
	return nil
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

	events := make(chan event.Event, 1)
	sub := subscriber{
		RWMutex: &sync.RWMutex{},
		ctx:     ctx,
		events:  events,
		done:    make(chan struct{}),
	}

	bus.mux.Lock()
	defer bus.mux.Unlock()
	for _, name := range names {
		if bus.subs[name] == nil {
			bus.subs[name] = make(map[subscriber]bool)
		}
		bus.subs[name][sub] = true
	}

	// unsubscribe when ctx canceled
	go func() {
		defer bus.unsubscribe(sub, names...)
		<-ctx.Done()
	}()

	return events, nil
}

func (bus *eventBus) run() {
	for evt := range bus.queue {
		go bus.publish(evt)
	}
}

func (bus *eventBus) publish(evt event.Event) {
	subs := bus.subscribers(evt.Name())
	for _, sub := range subs {
		go func(sub subscriber) {
			select {
			case <-sub.done:
				return
			default:
			}
			sub.RLock()
			defer sub.RUnlock()
			select {
			case <-sub.done:
			case sub.events <- evt:
			}
		}(sub)
	}
}

func (bus *eventBus) subscribers(name string) []subscriber {
	bus.mux.RLock()
	defer bus.mux.RUnlock()
	subs := make([]subscriber, 0, len(bus.subs[name]))
	for sub := range bus.subs[name] {
		subs = append(subs, sub)
	}
	return subs
}

func (bus *eventBus) unsubscribe(sub subscriber, names ...string) {
	defer func() {
		sub.Lock()
		defer sub.Unlock()
		close(sub.done)
		close(sub.events)
	}()

	bus.mux.Lock()
	defer bus.mux.Unlock()
	for _, name := range names {
		delete(bus.subs[name], sub)
	}
}
