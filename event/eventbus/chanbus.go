package eventbus

import (
	"context"
	"fmt"
	"sync"

	"github.com/modernice/goes/event"
)

type chanbus struct {
	sync.RWMutex

	events map[string]*eventSubscription
	queue  chan event.Event
	done   chan struct{}
}

type eventSubscription struct {
	bus        *chanbus
	recipients []recipient

	subscribeQueue   chan subscribeJob
	unsubscribeQueue chan subscribeJob
	events           chan event.Event

	done chan struct{}
}

type recipient struct {
	events   chan event.Event
	errs     chan error
	unsubbed chan struct{}
}

type subscribeJob struct {
	rcpt recipient
	done chan struct{}
}

func New() event.Bus {
	bus := &chanbus{
		events: make(map[string]*eventSubscription),
		queue:  make(chan event.Event),
	}
	go bus.work()
	return bus
}

func (bus *chanbus) Subscribe(ctx context.Context, events ...string) (<-chan event.Event, <-chan error, error) {
	ctx, unsubscribeAll := context.WithCancel(ctx)
	go func() {
		// Will never happen, but makes the linter happy.
		<-bus.done
		unsubscribeAll()
	}()

	var rcpts []recipient

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

func (bus *chanbus) Publish(ctx context.Context, events ...event.Event) error {
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

func (bus *chanbus) subscribe(ctx context.Context, name string) (recipient, error) {
	bus.Lock()
	defer bus.Unlock()

	if sub, ok := bus.events[name]; ok {
		rcpt, err := sub.subscribe(ctx)
		if err != nil {
			return rcpt, fmt.Errorf("add recipient: %w [event=%v]", err, name)
		}
		return rcpt, nil
	}

	sub := &eventSubscription{
		bus:              bus,
		subscribeQueue:   make(chan subscribeJob),
		unsubscribeQueue: make(chan subscribeJob),
		events:           make(chan event.Event),
		done:             make(chan struct{}),
	}
	bus.events[name] = sub

	go sub.work()

	rcpt, err := sub.subscribe(ctx)
	if err != nil {
		return recipient{}, fmt.Errorf("add recipient: %w [event=%v]", err, name)
	}

	return rcpt, nil
}

func (bus *chanbus) work() {
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

func (sub *eventSubscription) subscribe(ctx context.Context) (recipient, error) {
	rcpt := recipient{
		events:   make(chan event.Event),
		errs:     make(chan error),
		unsubbed: make(chan struct{}),
	}

	job := subscribeJob{
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
			sub.unsubscribeQueue <- subscribeJob{
				rcpt: rcpt,
				done: make(chan struct{}),
			}
		}()
	}()

	select {
	case <-ctx.Done():
		return recipient{}, ctx.Err()
	case <-job.done:
		return rcpt, nil
	}
}

func (sub *eventSubscription) work() {
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

func fanInEvents(ctx context.Context, rcpts []recipient) <-chan event.Event {
	out := make(chan event.Event)
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

func fanInErrors(ctx context.Context, rcpts []recipient) <-chan error {
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
