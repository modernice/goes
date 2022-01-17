package eventbus

import (
	"context"
	"fmt"
	"sync"

	"github.com/modernice/goes/event"
)

type chanbus[D any] struct {
	sync.RWMutex

	events map[string]*eventSubscription[D]
	queue  chan event.Event[D]
	done   chan struct{}
}

type eventSubscription[D any] struct {
	bus        *chanbus[D]
	recipients []recipient[D]

	subscribeQueue   chan subscribeJob[D]
	unsubscribeQueue chan subscribeJob[D]
	events           chan event.Event[D]

	done chan struct{}
}

type recipient[D any] struct {
	events   chan event.Event[D]
	errs     chan error
	unsubbed chan struct{}
}

type subscribeJob[D any] struct {
	rcpt recipient[D]
	done chan struct{}
}

func New[D any]() event.Bus[D] {
	bus := &chanbus[D]{
		events: make(map[string]*eventSubscription[D]),
		queue:  make(chan event.Event[D]),
	}
	go bus.work()
	return bus
}

func (bus *chanbus[D]) Subscribe(ctx context.Context, events ...string) (<-chan event.Event[D], <-chan error, error) {
	ctx, unsubscribeAll := context.WithCancel(ctx)
	go func() {
		// Will never happen, but makes the linter happy.
		<-bus.done
		unsubscribeAll()
	}()

	var rcpts []recipient[D]

	for _, name := range events {
		rcpt, err := bus.subscribe(ctx, name)
		if err != nil {
			unsubscribeAll()
			return nil, nil, err
		}
		rcpts = append(rcpts, rcpt)
	}

	return fanInEvents(ctx, rcpts), fanInErrors(ctx, rcpts), nil
}

func (bus *chanbus[D]) Publish(ctx context.Context, events ...event.Event[D]) error {
	done := make(chan struct{})
	go func() {
		defer close(done)
		for _, evt := range events {
			select {
			case <-ctx.Done():
				return
			case bus.queue <- evt:
			}
		}
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		return nil
	}
}

func (bus *chanbus[D]) subscribe(ctx context.Context, name string) (recipient[D], error) {
	bus.Lock()
	defer bus.Unlock()

	if sub, ok := bus.events[name]; ok {
		rcpt, err := sub.subscribe(ctx)
		if err != nil {
			return rcpt, fmt.Errorf("add recipient: %w [event=%v]", err, name)
		}
		return rcpt, nil
	}

	sub := &eventSubscription[D]{
		bus:              bus,
		subscribeQueue:   make(chan subscribeJob[D]),
		unsubscribeQueue: make(chan subscribeJob[D]),
		events:           make(chan event.Event[D]),
		done:             make(chan struct{}),
	}
	bus.events[name] = sub

	go sub.work()

	rcpt, err := sub.subscribe(ctx)
	if err != nil {
		return recipient[D]{}, fmt.Errorf("add recipient: %w [event=%v]", err, name)
	}

	return rcpt, nil
}

func (bus *chanbus[D]) work() {
	for evt := range bus.queue {
		bus.RLock()
		sub, ok := bus.events[evt.Name()]
		bus.RUnlock()
		if !ok {
			continue
		}
		sub.events <- evt
	}
}

func (sub *eventSubscription[D]) subscribe(ctx context.Context) (recipient[D], error) {
	rcpt := recipient[D]{
		events:   make(chan event.Event[D]),
		errs:     make(chan error),
		unsubbed: make(chan struct{}),
	}

	job := subscribeJob[D]{
		rcpt: rcpt,
		done: make(chan struct{}),
	}

	go func() {
		select {
		case <-ctx.Done():
			return
		case sub.subscribeQueue <- job:
		}

		go func() {
			<-ctx.Done()
			close(rcpt.unsubbed)
			sub.unsubscribeQueue <- subscribeJob[D]{
				rcpt: rcpt,
				done: make(chan struct{}),
			}
		}()
	}()

	select {
	case <-ctx.Done():
		return recipient[D]{}, ctx.Err()
	case <-job.done:
		return rcpt, nil
	}
}

func (sub *eventSubscription[D]) work() {
	for {
		select {
		case job := <-sub.subscribeQueue:
			sub.recipients = append(sub.recipients, job.rcpt)
			close(job.done)
		case job := <-sub.unsubscribeQueue:
			close(job.rcpt.events)
			close(job.rcpt.errs)
			for i, rcpt := range sub.recipients {
				if rcpt == job.rcpt {
					sub.recipients = append(sub.recipients[:i], sub.recipients[i+1:]...)
					break
				}
			}
			close(job.done)
		case evt := <-sub.events:
			for _, rcpt := range sub.recipients {
				select {
				case <-rcpt.unsubbed:
				case rcpt.events <- evt:
				}
			}
		}
	}
}

func fanInEvents[D any](ctx context.Context, rcpts []recipient[D]) <-chan event.Event[D] {
	out := make(chan event.Event[D])
	var wg sync.WaitGroup
	wg.Add(len(rcpts))
	go func() {
		wg.Wait()
		close(out)
	}()
	for _, rcpt := range rcpts {
		rcpt := rcpt
		go func() {
			defer wg.Done()
			for evt := range rcpt.events {
				select {
				case <-ctx.Done():
					return
				case out <- evt:
				}
			}
		}()
	}
	return out
}

func fanInErrors[D any](ctx context.Context, rcpts []recipient[D]) <-chan error {
	out := make(chan error)
	var wg sync.WaitGroup
	wg.Add(len(rcpts))
	go func() {
		wg.Wait()
		close(out)
	}()
	for _, rcpt := range rcpts {
		rcpt := rcpt
		go func() {
			defer wg.Done()
			for err := range rcpt.errs {
				select {
				case <-ctx.Done():
					return
				case out <- err:
				}
			}
		}()
	}
	return out
}
