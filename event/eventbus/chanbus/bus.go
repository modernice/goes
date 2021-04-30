package chanbus

import (
	"context"
	"sync"

	"github.com/modernice/goes/event"
)

type bus struct {
	mux  sync.Mutex
	subs map[string][]*subscription
}

type subscription struct {
	ctx context.Context
	wg  *sync.WaitGroup
	out chan event.Event
}

func New() event.Bus {
	return &bus{
		subs: map[string][]*subscription{},
	}
}

func (b *bus) Subscribe(ctx context.Context, eventNames ...string) (<-chan event.Event, <-chan error, error) {
	out := make(chan event.Event)
	errs := make(chan error)
	var wg sync.WaitGroup
	wg.Add(len(eventNames))
	for _, name := range eventNames {
		b.subscribe(ctx, name, out, errs, &wg)
	}

	go func() {
		wg.Wait()
		close(errs)
		close(out)
	}()

	return out, errs, nil
}

func (b *bus) subscribe(ctx context.Context, name string, out chan<- event.Event, errs chan<- error, wg *sync.WaitGroup) {
	sub := &subscription{
		ctx: ctx,
		wg:  wg,
		out: make(chan event.Event),
	}
	b.mux.Lock()
	b.subs[name] = append(b.subs[name], sub)
	b.mux.Unlock()

	go func() {
		<-ctx.Done()
		b.removeSub(name, sub)
		close(sub.out)
	}()

	go func() {
		defer wg.Done()
		for evt := range sub.out {
			select {
			case <-sub.ctx.Done():
			case out <- evt:
			}
		}
	}()
}

func (b *bus) Publish(ctx context.Context, events ...event.Event) error {
	for _, evt := range events {
		b.publish(ctx, evt)
	}
	return nil
}

func (b *bus) publish(ctx context.Context, evt event.Event) {
	subs := b.subscribers(evt.Name())
	for _, sub := range subs {
		select {
		case <-sub.ctx.Done():
		case sub.out <- evt:
		}
	}
}

func (b *bus) subscribers(name string) []*subscription {
	b.mux.Lock()
	defer b.mux.Unlock()
	subs := make([]*subscription, len(b.subs[name]))
	copy(subs, b.subs[name])
	return subs
}

func (b *bus) removeSub(eventName string, sub *subscription) {
	b.mux.Lock()
	defer b.mux.Unlock()
	for n, subs := range b.subs {
		if n != eventName {
			continue
		}
		for i, s := range subs {
			if s != sub {
				continue
			}
			b.subs[n] = append(b.subs[n][:i], b.subs[n][i+1:]...)
			return
		}
	}
}
