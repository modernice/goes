package errbus

import (
	"context"
	"sync"
)

// Bus is an error bus that can be used to provide an async error API.
type Bus struct {
	subsMux sync.RWMutex
	subs    []subscriber
	errs    chan error

	once sync.Once
}

type subscriber struct {
	ctx  context.Context
	errs chan error
}

// New returns a new Bus.
func New() *Bus {
	return &Bus{errs: make(chan error)}
}

// Publish publishes the error to all subscribers.
func (b *Bus) Publish(ctx context.Context, err error) error {
	b.init()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case b.errs <- err:
		return nil
	}
}

// Subscribe returns a channel of errors. When an error is published through
// b.Publish, that error will be received from the error channel.
func (b *Bus) Subscribe(ctx context.Context) <-chan error {
	b.init()
	sub := b.subscribe(ctx)
	return sub.errs
}

func (b *Bus) init() {
	b.once.Do(func() {
		go b.handleErrors()
	})
}

func (b *Bus) subscribe(ctx context.Context) subscriber {
	errs := make(chan error)
	sub := subscriber{
		ctx:  ctx,
		errs: errs,
	}
	b.subsMux.Lock()
	defer b.subsMux.Unlock()
	b.subs = append(b.subs, sub)
	go func() {
		<-ctx.Done()
		b.unsubscribe(sub)
	}()
	return sub
}

func (b *Bus) unsubscribe(sub subscriber) {
	defer close(sub.errs)
	b.subsMux.Lock()
	defer b.subsMux.Unlock()
	for i, bsub := range b.subs {
		if bsub == sub {
			b.subs = append(b.subs[:i], b.subs[i+1:]...)
			return
		}
	}
}

func (b *Bus) handleErrors() {
	for err := range b.errs {
		b.publish(err)
	}
}

func (b *Bus) publish(err error) {
	b.subsMux.RLock()
	defer b.subsMux.RUnlock()
	for _, sub := range b.subs {
		go func(sub subscriber) {
			select {
			case <-sub.ctx.Done():
			case sub.errs <- err:
			}
		}(sub)
	}
}
