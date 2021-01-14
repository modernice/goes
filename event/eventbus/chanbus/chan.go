package chanbus

import (
	"context"
	"sync"

	"github.com/modernice/goes/event"
)

type eventBus struct {
	mux   sync.RWMutex
	subs  map[string]map[chan event.Event]bool
	queue chan event.Event
}

// New returns a Bus that communicates over channels.
func New() event.Bus {
	bus := &eventBus{
		subs:  make(map[string]map[chan event.Event]bool),
		queue: make(chan event.Event),
	}
	go bus.run()
	return bus
}

// Publish sends the Event to the Event channels that have been returned by
// previous calls to bus.Subscribe() with evt.Name() as the Event name. If ctx
// is canceled, ctx.Err() is returned.
func (bus *eventBus) Publish(ctx context.Context, evt event.Event) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case bus.queue <- evt:
		return nil
	}
}

// Subscribe returns a channel of Events. For every published Event evt where
// evt.Name() is one of names, that Event will be received from the returned
// Events channel. When ctx is canceled, events will be closed.
func (bus *eventBus) Subscribe(ctx context.Context, names ...string) (<-chan event.Event, error) {
	events := make(chan event.Event, 1)
	bus.mux.Lock()
	defer bus.mux.Unlock()
	for _, name := range names {
		if bus.subs[name] == nil {
			bus.subs[name] = make(map[chan event.Event]bool)
		}
		bus.subs[name][events] = true
	}

	// unsubscribe when ctx canceled
	go func() {
		defer close(events)
		<-ctx.Done()
		bus.unsubscribe(events, names...)
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
	for _, events := range subs {
		events := events
		go func() { events <- evt }()
	}
}

func (bus *eventBus) subscribers(name string) []chan event.Event {
	bus.mux.RLock()
	defer bus.mux.RUnlock()
	subs := make([]chan event.Event, 0, len(bus.subs[name]))
	for sub := range bus.subs[name] {
		subs = append(subs, sub)
	}
	return subs
}

func (bus *eventBus) unsubscribe(sub chan event.Event, names ...string) {
	bus.mux.Lock()
	defer bus.mux.Unlock()
	for _, name := range names {
		delete(bus.subs[name], sub)
	}
}
